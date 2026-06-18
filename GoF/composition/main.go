package main

import "fmt"

type SystemF interface {
	Print(prefix string)
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

func main() {
	parent := &Folder{Name: "parent"}

	fileChild1 := &Folder{Name: "fileChild1"}

	folderChild1 := &Folder{Name: "folderChild1"}

	fileChild2 := &Folder{Name: "fileChild2"}

	parent.Chilren = append(parent.Chilren, fileChild1, folderChild1)
	folderChild1.Chilren = append(folderChild1.Chilren, fileChild2)

	parent.Print("")
}
