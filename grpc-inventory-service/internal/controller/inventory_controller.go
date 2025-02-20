package controller

import (
	"context"

	"github.com/gocql/gocql"
	"github.com/yaninyzwitty/grpc-inventory-service/pb"
)

type InventoryController struct {
	sesion *gocql.Session
	pb.UnimplementedInventoryServiceServer
}

func NewInventoryController(sesion *gocql.Session) *InventoryController {
	return &InventoryController{sesion: sesion}
}

func (c *InventoryController) GetInventory(ctx context.Context, req *pb.GetInventoryRequest) (*pb.GetInventoryResponse, error) {
	return &pb.GetInventoryResponse{}, nil
}

func (c *InventoryController) ListInventory(ctx context.Context, req *pb.ListInventoryRequest) (*pb.ListInventoryResponse, error) {
	return &pb.ListInventoryResponse{}, nil
}
func (c *InventoryController) UpdateInventory(ctx context.Context, req *pb.UpdateInventoryRequest) (*pb.UpdateInventoryResponse, error) {
	return &pb.UpdateInventoryResponse{}, nil
}
