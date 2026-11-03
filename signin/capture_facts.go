package signin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CaptureFactsDB intentionally exposes only the read operation needed by status.
type CaptureFactsDB interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// CaptureFactStore reads enrollment facts from an independently owned capture table.
// It cannot create, transition, or delete captures.
type CaptureFactStore struct {
	DB    CaptureFactsDB
	Table string
}

func captureOwnerKey(owner string) string {
	h := sha256.Sum256([]byte(owner))
	return hex.EncodeToString(h[:])
}
func (s CaptureFactStore) get(ctx context.Context, id string, out any) error {
	if s.DB == nil || s.Table == "" {
		return ErrCaptureNotFound
	}
	result, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(s.Table), Key: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}}, ConsistentRead: aws.Bool(true)})
	if err != nil {
		return err
	}
	if result == nil || len(result.Item) == 0 {
		return ErrCaptureNotFound
	}
	return attributevalue.UnmarshalMap(result.Item, out)
}
func (s CaptureFactStore) Account(ctx context.Context, owner string) (CaptureAccount, error) {
	if owner == "" {
		return CaptureAccount{}, ErrCaptureNotFound
	}
	var row struct {
		Owner      string
		IDCaptured bool
		CurrentID  string
	}
	err := s.get(ctx, "ACCOUNT#"+captureOwnerKey(owner), &row)
	if errors.Is(err, ErrCaptureNotFound) {
		return CaptureAccount{}, nil
	}
	if err != nil {
		return CaptureAccount{}, err
	}
	if row.Owner != owner {
		return CaptureAccount{}, ErrCaptureNotFound
	}
	return CaptureAccount{IDCaptured: row.IDCaptured, CurrentID: row.CurrentID}, nil
}
func (s CaptureFactStore) Get(ctx context.Context, owner, id string) (CaptureRecord, error) {
	if owner == "" || id == "" {
		return CaptureRecord{}, ErrCaptureNotFound
	}
	var row struct {
		Owner string
		State string
	}
	if err := s.get(ctx, "CAP#"+captureOwnerKey(owner)+"#"+id, &row); err != nil {
		return CaptureRecord{}, err
	}
	if row.Owner != owner {
		return CaptureRecord{}, ErrCaptureNotFound
	}
	return CaptureRecord{State: row.State}, nil
}
