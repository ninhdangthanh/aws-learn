package main

import "fmt"

type AuthHandler struct {
	BaseHandler
}

func (a *AuthHandler) Handle(req *Request) {
	fmt.Println("Handle check auth token...")

	if req.Token == "" {
		fmt.Println("UNAUTHORIZED")
		return
	}

	a.BaseHandler.Handle(req)
}

type PermissionHandler struct {
	BaseHandler
}

func (p *PermissionHandler) Handle(req *Request) {
	fmt.Println("Handle check permission...")

	if req.Role != "ADMIN" {
		fmt.Println("FORBIDDEN")
		return
	}

	p.BaseHandler.Handle(req)
}
