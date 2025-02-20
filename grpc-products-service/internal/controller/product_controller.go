package controller

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gocql/gocql"
	"github.com/grpc-products-service/pb"
	"github.com/grpc-products-service/snowflake"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	CREATE_PRODUCT_EVENT = "CREATE_PRODUCT_EVENT"
)

type ProductController struct {
	session *gocql.Session
	pb.UnimplementedProductServiceServer
}

func NewProductController(session *gocql.Session) *ProductController {
	return &ProductController{
		session: session,
	}
}

func (c *ProductController) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	batch := `BEGIN BATCH
		INSERT INTO eccomerce_keyspace.products (id, name, description, price, category, tags, stock_count, created_at, updated_at) 
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?);
		INSERT INTO eccomerce_keyspace.products_outbox (bucket, id, event_type, payload, processed, created_at) 
		VALUES(?, ?, ?, ?, ?, ?);
	APPLY BATCH;`

	productId, err := snowflake.GenerateID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate product id: %v", err)
	}

	now := time.Now()
	outboxId := gocql.TimeUUID()
	bucket := time.Now().Format("2006-01-02")

	product := &pb.Product{
		Id:          int64(productId),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Price:       req.GetPrice(),
		Category:    req.GetCategory(),
		Tags:        req.GetTags(),
		StockCount:  0,
		CreatedAt:   timestamppb.New(now),
		UpdatedAt:   timestamppb.New(now),
	}

	// Marshal struct to JSON
	payload, err := json.Marshal(product)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal product: %v", err)
	}

	err = c.session.Query(batch,
		productId, product.Name, product.Description, product.Price, product.Category, product.Tags, product.StockCount, now, now,
		bucket, outboxId, CREATE_PRODUCT_EVENT, string(payload), false, now,
	).WithContext(ctx).Exec()

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute batch query: %v", err)
	}

	return &pb.CreateProductResponse{
		Product: product,
	}, nil
}

func (c *ProductController) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	var product pb.Product
	var createdAt, updatedAt time.Time
	query := `SELECT id, name, description, price, category, tags, stock_count, created_at, updated_at 
			  FROM eccomerce_keyspace.products WHERE id = ? LIMIT 1`

	err := c.session.Query(query, req.GetId()).WithContext(ctx).Consistency(gocql.Quorum).Scan(
		&product.Id, &product.Name, &product.Description, &product.Price,
		&product.Category, &product.Tags, &product.StockCount, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == gocql.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "product not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to fetch product: %v", err)
	}

	product.CreatedAt = timestamppb.New(createdAt)
	product.UpdatedAt = timestamppb.New(updatedAt)
	return &pb.GetProductResponse{
		Product: &product,
	}, nil
}

func (c *ProductController) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	return &pb.ListProductsResponse{}, nil
}

func (c *ProductController) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.UpdateProductResponse, error) {
	return &pb.UpdateProductResponse{}, nil
}
func (c *ProductController) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.DeleteProductResponse, error) {
	return &pb.DeleteProductResponse{}, nil
}
