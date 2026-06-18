package main

func main() {
	auth := &AuthHandler{}

	permissionHandler := &PermissionHandler{}

	auth.SetNext(permissionHandler)

	req := &Request{
		User:    "ninh",
		Token:   "abc123",
		Role:    "ADMIN",
		OrderID: "ORD001",
	}

	auth.Handle(req)

}
