package main

type Request struct {
	User    string
	Token   string
	Role    string
	OrderID string
}

type Handler interface {
	SetNext(Handler) Handler
	Handle(*Request)
}

type BaseHandler struct {
	next Handler
}

func (h *BaseHandler) SetNext(next Handler) Handler {
	h.next = next
	return next
}

func (h *BaseHandler) Handle(req *Request) {
	if h.next != nil {
		h.next.Handle(req)
	}
}
