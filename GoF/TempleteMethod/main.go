package main

func main() {
	base := BaseReport{}

	pdf := &PDFReport{}
	base.Generator = pdf

	base.Generate()

	excel := &ExcelReport{}
	base.Generator = excel

	base.Generate()
}
