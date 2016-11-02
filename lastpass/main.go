package main

import (
	"encoding/csv"
	"encoding/xml"
	"io"
	"log"
	"os"
	"time"
)

// convert LastPass export to KeePass 1.0 XML format
func main() {
	r := csv.NewReader(os.Stdin)
	head, err := r.Read()
	// `url,username,password,extra,name,grouping,fav`
	if err != nil {
		log.Fatal(err)
	}
	order := make(map[string]int, 7)
	for i, nm := range head {
		switch nm {
		case "url", "username", "password", "extra", "name", "grouping", "fav":
			order[nm] = i
		default:
			log.Printf("WARN: unknown field %q", nm)
		}
	}
	w := io.WriteCloser(os.Stdout)
	defer w.Close()
	io.WriteString(w, "<!DOCTYPE KEEPASSX_DATABASE>\n<database>\n<group><title>Import</title><icon>1</icon>\n")
	defer io.WriteString(w, "</group></database>")

	enc := xml.NewEncoder(w)
	defer enc.Flush()
	enc.Indent("", "  ")

	for {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		log.Printf("ROW=%q", row)
		var rec entry
		for nm, i := range order {
			switch nm {
			case "url":
				rec.URL = row[i]
			case "username":
				rec.Username = row[i]
			case "password":
				rec.Password = row[i]
			case "extra":
				rec.Comment = row[i]
			case "name":
				rec.Title = row[i]
				//case "grouping":
				//rec.Group = row[i]
			}
		}
		rec.Expire = NotExpire

		if err = enc.Encode(rec); err != nil {
			log.Fatal(err)
		}
		enc.Flush()
	}
}

type entry struct {
	XMLName    string    `xml:"entry"`
	Title      string    `xml:"title"`
	Username   string    `xml:"username"`
	URL        string    `xml:"url"`
	Password   string    `xml:"password"`
	Comment    string    `xml:"comment"`
	Icon       int       `xml:"icon"`
	Creation   time.Time `xml:"creation"`
	LastMod    time.Time `xml:"lastmod"`
	LastAccess time.Time `xml:"lastaccess"`
	Expire     string    `xml:"expire"`
}

const NotExpire = "Never"
