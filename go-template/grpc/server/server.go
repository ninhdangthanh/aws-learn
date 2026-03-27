package server

import (
	"context"
	"log"
	"net"

	"github.com/go-template/grpc/order"
	"github.com/go-template/grpc/product"
	"github.com/go-template/grpc/user"
	"github.com/go-template/service"

	"google.golang.org/grpc"
)

type orderServer struct {
	order.UnimplementedOrderServiceServer
	svc service.OrderService
}

func (s *orderServer) GetOrder(ctx context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	ord, err := s.svc.GetOrder(ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	return &order.GetOrderResponse{
		Id:       uint32(ord.ID),
		UserId:   ord.UserID,
		ItemName: ord.ItemName,
		Amount:   ord.Amount,
		Status:   ord.Status,
	}, nil
}

type userServer struct {
	user.UnimplementedUserServiceServer
	svc service.UserService
}

func (s *userServer) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.GetUserResponse, error) {
	usr, err := s.svc.GetUser(ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	return &user.GetUserResponse{
		Id:    uint32(usr.ID),
		Email: usr.Email,
		Name:  usr.Name,
	}, nil
}

type productServer struct {
	product.UnimplementedProductServiceServer
	svc service.ProductService
}

func (s *productServer) GetProduct(ctx context.Context, req *product.GetProductRequest) (*product.GetProductResponse, error) {
	p, err := s.svc.GetProduct(ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	return &product.GetProductResponse{
		Id:    uint32(p.ID),
		Sku:   p.SKU,
		Name:  p.Name,
		Price: p.Price,
		Stock: int32(p.Stock),
	}, nil
}

func StartGRPCServer(port string) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Register Services
	order.RegisterOrderServiceServer(grpcServer, &orderServer{svc: service.NewOrderService()})
	user.RegisterUserServiceServer(grpcServer, &userServer{svc: service.NewUserService()})
	product.RegisterProductServiceServer(grpcServer, &productServer{svc: service.NewProductService()})

	log.Printf("Starting gRPC server on %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
