## wrap

wrap is a golang package for text wrapping (or text re-wrapping).

(You might also be interested in the [wrap function in godoc](https://twitter.com/jbeda/status/560499368344576001).)

By Brandon Thomson &lt;bt@brandonthomson.com&gt;<br>
[www.brandonthomson.com](https://www.brandonthomson.com)

2-clause BSD license.

## Examples

```go
s := "Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."

fmt.Println(wrap.String(s, 80))
```

```
Lorem ipsum dolor sit amet, consectetur adipisicing elit, sed do eiusmod tempor
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in
culpa qui officia deserunt mollit anim id est laborum.
```


```go
fmt.Println(wrap.String(s, 40))
```

```
Lorem ipsum dolor sit amet, consectetur
adipisicing elit, sed do eiusmod tempor
incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis
nostrud exercitation ullamco laboris
nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit
in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint
occaecat cupidatat non proident, sunt in
culpa qui officia deserunt mollit anim
id est laborum.
```

## Caveats

- Output will use \n (newline) characters to separate lines.

- ASCII space (" "), ASCII newline ("\n"), and ASCII carriage return ("\r") are
  all considered to be whitespace and will trigger line breaks. ASCII Tab and
  other kinds of unicode whitespace characters are not considered whitespace
  and will not cause breaks. Notably the ASCII hyphen ("-") is not considered
  whitespace, and so no breaking will occur at a dash under normal
  circumstances. These behaviors should probably be runtime-configurable but
  they are not in the current version of this package. Modify the source as
  needed to suit your circumstances.

- Normally line breaks will only be inserted at word boundaries. However,
  really long words which cannot fit onto a single line will be split inside
  the word. To use a silly example, if your string is 123456 and you specify a
  column size of 2, then you will receive this output:

```
12
34
56
```

- Newlines and carriage returns in the input will not be preserved, which means
  that if you pass in text containing several paragraphs you'll get one giant
  paragraph back. If you wish to avoid that, then split your text into
  paragraphs and call `wrap.String()` individually on each paragraph.

- "column count" is actually the number of unicode runes, which may have
  unintended results for languages which 1) use multiple unicode runes to
  produce a character printable in a single column, or 2) use a single unicode
  rune to produce a "wide" (2-column) character (I believe Japanese and Chinese
  are common examples here). I don't plan to fix these issues so you should
  fork this package if this presents a problem for you.

## Installation

I don't use `go get` to download packages, so this package has not been tested
with `go get`. I imagine it should work correctly, but please report any
problems and I'll fix them.

## Tests

Yes, there are quite a few of them:

```go
{"", "", 1},
{"", "", 2},
{"", "", 3},

{"1  4", "1 4", 10},

{"1   5", "1 5", 10},

{"1    6", "1 6", 10},
{"1 \n 6", "1 6", 10},
{"1 \r 6", "1 6", 10},
{"1\r\n6", "1 6", 10},
{"1\n\r6", "1 6", 10},

{"123456789", "123456789", 0},

{"123456", "1\n2\n3\n4\n5\n6", 1},
{"123456", "12\n34\n56", 2},
{"123456", "123\n456", 3},
{"123456", "1234\n56", 4},
{"123456", "12345\n6", 5},
{"123456", "123456", 6},
{"123456", "123456", 7},

{"1 3", "1\n3", 1},
{"1 3", "1\n3", 2},
{"1 3", "1 3", 3},
{"1 3", "1 3", 4},

{"12 45", "1\n2\n4\n5", 1},
{"12 45", "12\n45", 2},
{"12 45", "12\n45", 3},
{"12 45", "12\n45", 4},
{"12 45", "12 45", 5},
{"12 45", "12 45", 6},

{"123 567", "1\n2\n3\n5\n6\n7", 1},
{"123 567", "12\n3\n56\n7", 2},
{"123 567", "123\n567", 3},
{"123 567", "123\n567", 4},
{"123 567", "123\n567", 5},
{"123 567", "123\n567", 6},
{"123 567", "123 567", 7},
{"123 567", "123 567", 8},

{"123 567 9ab", "1\n2\n3\n5\n6\n7\n9\na\nb", 1},
{"123 567 9ab", "12\n3\n56\n7\n9a\nb", 2},
{"123 567 9ab", "123\n567\n9ab", 3},
{"123 567 9ab", "123\n567\n9ab", 4},
{"123 567 9ab", "123\n567\n9ab", 5},
{"123 567 9ab", "123\n567\n9ab", 6},
{"123 567 9ab", "123 567\n9ab", 7},
{"123 567 9ab", "123 567\n9ab", 8},
{"123 567 9ab", "123 567\n9ab", 9},
{"123 567 9ab", "123 567\n9ab", 10},
{"123 567 9ab", "123 567 9ab", 11},
{"123 567 9ab", "123 567 9ab", 12},

{"123 567 9ab def", "1\n2\n3\n5\n6\n7\n9\na\nb\nd\ne\nf", 1},
{"123 567 9ab def", "12\n3\n56\n7\n9a\nb\nde\nf", 2},
{"123 567 9ab def", "123\n567\n9ab\ndef", 3},
{"123 567 9ab def", "123\n567\n9ab\ndef", 4},
{"123 567 9ab def", "123\n567\n9ab\ndef", 5},
{"123 567 9ab def", "123\n567\n9ab\ndef", 6},
{"123 567 9ab def", "123 567\n9ab def", 7},
{"123 567 9ab def", "123 567\n9ab def", 8},
{"123 567 9ab def", "123 567\n9ab def", 9},
{"123 567 9ab def", "123 567\n9ab def", 10},
{"123 567 9ab def", "123 567 9ab\ndef", 11},
{"123 567 9ab def", "123 567 9ab\ndef", 12},
{"123 567 9ab def", "123 567 9ab\ndef", 13},
{"123 567 9ab def", "123 567 9ab\ndef", 14},
{"123 567 9ab def", "123 567 9ab def", 15},
{"123 567 9ab def", "123 567 9ab def", 16},

{"1\n2\n3", "1 2 3", 10},
{"1\r\n2\r\n3", "1 2 3", 10},
{"1\n\r2\n\r3", "1 2 3", 10},

{"1\n2\n3\n4", "1 2\n3 4", 3},
{"1\r\n2\r\n3\r\n4", "1 2\n3 4", 3},
{"1 \r\n 2 \r\n 3 \r\n 4", "1 2\n3 4", 3},
{"1 \r\n2 \r\n3 \r\n4", "1 2\n3 4", 3},
{"1\n\r2\n\r3\n\r4", "1 2\n3 4", 3},
{"1 \n\r 2 \n\r 3 \n\r 4", "1 2\n3 4", 3},
{"1\n\r 2\n\r 3\n\r 4", "1 2\n3 4", 3},
{"1 \n\r2 \n\r3 \n\r4", "1 2\n3 4", 3},

{"1\n2\n3\n4", "1 2 3 4", 10},
{"1\r\n2\r\n3\r\n4", "1 2 3 4", 10},
```
