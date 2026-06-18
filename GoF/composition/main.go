package main

import "fmt"

type SystemF interface {
	Print(prefix string)
	CalSize() int
}

type Folder struct {
	Name    string
	Chilren []SystemF
}

func (f *Folder) Print(prefix string) {
	fmt.Println(prefix + "---- " + f.Name)

	for _, c := range f.Chilren {
		c.Print(prefix + "   ")
	}
}

func (f *Folder) CalSize() int {
	size := 1

	for _, c := range f.Chilren {
		size += c.CalSize()
	}

	return size
}

func main() {
	parent := &Folder{Name: "parent"}

	fileChild1 := &Folder{Name: "fileChild1"}

	folderChild1 := &Folder{Name: "folderChild1"}

	fileChild2 := &Folder{Name: "fileChild2"}

	parent.Chilren = append(parent.Chilren, fileChild1, folderChild1)
	folderChild1.Chilren = append(folderChild1.Chilren, fileChild2)

	fmt.Println("all size: ", parent.CalSize())
	parent.Print("")
}
