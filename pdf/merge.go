package pdf

import (
	"fmt"
	"io"

	"github.com/nathankerr/pdf"
	"gopkg.in/inconshreveable/log15.v2"
)

var Log = log15.New("lib", "pdf")

func MergeFiles(dest string, sources ...string) error {
	merged, err := pdf.Create(dest)
	if err != nil {
		return fmt.Errorf("create %q: %v", dest, err)
	}

	// because pdf files are mmap'ed and objects are zero copied
	// the files must remain open until merged is saved
	closers := make([]io.Closer, 0, len(sources))
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	// add the contents of each pdf into the merged pdf
	// collects the roots of each pdf
	roots := make([]pdf.ObjectReference, 0, len(sources))
	for _, fn := range sources {
		file, err := pdf.Open(fn)
		if err != nil {
			return fmt.Errorf("open %q: %v", fn, err)
		}
		closers = append(closers, file)

		_, root, err := copyReferencedObjects(map[pdf.ObjectReference]pdf.ObjectReference{}, merged, file, file.Root)
		if err != nil {
			return err
		}
		roots = append(roots, root.(pdf.ObjectReference))
		merged.Root = root.(pdf.ObjectReference)
	}

	// get the catalogs for each of the pdfs for merging their contents
	catalogs := make([]pdf.Dictionary, 0, len(roots))
	for _, root := range roots {
		catalogs = append(catalogs, merged.Get(root).(pdf.Dictionary))
	}

	// merge the page trees
	pageTreeRef, err := mergePageTrees(merged, catalogs)
	if err != nil {
		return err
	}

	// create a new root
	merged.Root, err = merged.Add(pdf.Dictionary{
		"Type":  pdf.Name("Catalog"),
		"Pages": pageTreeRef,
	})
	if err != nil {
		return err
	}

	return merged.Save()
}

func copyReferencedObjects(refs map[pdf.ObjectReference]pdf.ObjectReference, dst, src *pdf.File, obj pdf.Object) (map[pdf.ObjectReference]pdf.ObjectReference, pdf.Object, error) {
	var merge = func(newRefs map[pdf.ObjectReference]pdf.ObjectReference) {
		for k, v := range newRefs {
			refs[k] = v
		}
	}
	var err error
	switch t := obj.(type) {
	case pdf.ObjectReference:
		if _, ok := refs[t]; ok {
			obj = refs[t]
			break
		}

		// get an object reference for the copied obj
		// needed to break reference cycles
		ref, err := dst.Add(pdf.Null{})
		if err != nil {
			return nil, nil, err
		}
		refs[t] = ref

		newRefs, newObj, err := copyReferencedObjects(refs, dst, src, src.Get(t))
		if err != nil {
			return nil, nil, err
		}
		merge(newRefs)

		// now actually add the object to dst
		if refs[t], err = dst.Add(pdf.IndirectObject{
			ObjectReference: ref,
			Object:          newObj,
		}); err != nil {
			return nil, nil, err
		}

		obj = refs[t]
	case pdf.Dictionary:
		for k, v := range t {
			var newRefs map[pdf.ObjectReference]pdf.ObjectReference
			if newRefs, t[k], err = copyReferencedObjects(refs, dst, src, v); err != nil {
				return nil, nil, err
			}

			merge(newRefs)
		}
		obj = t
	case pdf.Array:
		for i, v := range t {
			var newRefs map[pdf.ObjectReference]pdf.ObjectReference
			if newRefs, t[i], err = copyReferencedObjects(refs, dst, src, v); err != nil {
				return nil, nil, err
			}
			merge(newRefs)
		}
		obj = t
	case pdf.Stream:
		for k, v := range t.Dictionary {
			var newRefs map[pdf.ObjectReference]pdf.ObjectReference
			if newRefs, t.Dictionary[k], err = copyReferencedObjects(refs, dst, src, v); err != nil {
				return nil, nil, err
			}
			merge(newRefs)
		}
		obj = t
	case pdf.Name, pdf.Integer, pdf.String, pdf.Real:
		// these types can't have references
	default:
		return nil, nil, fmt.Errorf("unhandled %T", obj)
	}

	return refs, obj, nil
}

func mergePageTrees(file *pdf.File, catalogs []pdf.Dictionary) (pdf.ObjectReference, error) {
	// reserve a reference for the new page tree root
	// needed to set the parent for the old page tree roots
	pageTreeRef, err := file.Add(pdf.Null{})
	if err != nil {
		return pageTreeRef, err
	}

	// use the old page tree roots as our page tree kids
	kids := pdf.Array{}
	pageCount := pdf.Integer(0)
	for _, catalog := range catalogs {
		// add the old page tree root to our list of kids
		pagesRef := catalog["Pages"].(pdf.ObjectReference)
		kids = append(kids, pagesRef)

		// now that the old page tree root is a kid, it needs a parent
		pages := file.Get(pagesRef).(pdf.Dictionary)
		pages["Parent"] = pageTreeRef
		_, err = file.Add(pdf.IndirectObject{
			ObjectReference: pagesRef,
			Object:          pages,
		})
		if err != nil {
			return pageTreeRef, err
		}

		pageCount += pages["Count"].(pdf.Integer)
	}

	// create the merged page tree
	_, err = file.Add(pdf.IndirectObject{
		ObjectReference: pageTreeRef,
		Object: pdf.Dictionary{
			"Type":  pdf.Name("Pages"),
			"Kids":  kids,
			"Count": pageCount,
		},
	})
	if err != nil {
		return pageTreeRef, err
	}

	return pageTreeRef, nil
}
