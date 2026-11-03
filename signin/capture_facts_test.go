package signin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/lambdawalker/go.apexfission.aws.auth/runtimeconfig"
)

type captureFactsDB struct {
	item  map[string]types.AttributeValue
	input *dynamodb.GetItemInput
	err   error
}

func (d *captureFactsDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	d.input = in
	return &dynamodb.GetItemOutput{Item: d.item}, d.err
}
func TestCaptureFactsReadLegacyKeysWithOwnerBinding(t *testing.T) {
	ctx := context.Background()
	hash := sha256.Sum256([]byte("subject"))
	ownerHash := hex.EncodeToString(hash[:])
	db := &captureFactsDB{item: map[string]types.AttributeValue{"Owner": &types.AttributeValueMemberS{Value: "subject"}, "IDCaptured": &types.AttributeValueMemberBOOL{Value: true}, "CurrentID": &types.AttributeValueMemberS{Value: "capture-id"}}}
	store := CaptureFactStore{DB: db, Table: "external-captures"}
	a, err := store.Account(ctx, "subject")
	if err != nil || !a.IDCaptured || a.CurrentID != "capture-id" {
		t.Fatal(a, err)
	}
	if aws.ToString(db.input.TableName) != "external-captures" || !aws.ToBool(db.input.ConsistentRead) || db.input.Key["id"].(*types.AttributeValueMemberS).Value != "ACCOUNT#"+ownerHash {
		t.Fatal(db.input)
	}
	db.item = map[string]types.AttributeValue{"Owner": &types.AttributeValueMemberS{Value: "subject"}, "State": &types.AttributeValueMemberS{Value: "ready"}}
	r, err := store.Get(ctx, "subject", "capture-id")
	if err != nil || r.State != "ready" || !aws.ToBool(db.input.ConsistentRead) || db.input.Key["id"].(*types.AttributeValueMemberS).Value != "CAP#"+ownerHash+"#capture-id" {
		t.Fatal(r, err, db.input)
	}
	db.item["Owner"] = &types.AttributeValueMemberS{Value: "another-subject"}
	if _, err := store.Account(ctx, "subject"); !errors.Is(err, ErrCaptureNotFound) {
		t.Fatal("cross-owner account exposed", err)
	}
	if _, err := store.Get(ctx, "subject", "capture-id"); !errors.Is(err, ErrCaptureNotFound) {
		t.Fatal("cross-owner capture exposed", err)
	}
	db.item = nil
	if a, err := store.Account(ctx, "subject"); err != nil || a.IDCaptured || a.CurrentID != "" {
		t.Fatal(a, err)
	}
	if _, err := store.Get(ctx, "subject", "capture-id"); !errors.Is(err, ErrCaptureNotFound) {
		t.Fatal(err)
	}
	db.err = errors.New("unavailable")
	if _, err := store.Account(ctx, "subject"); err == nil {
		t.Fatal("database failure hidden")
	}
}
func TestCapturelessEnrollmentNeverInventsCaptureCompletion(t *testing.T) {
	s, id, cfg, facts := enrollmentFixture()
	s.Captures = nil
	id.passkey = true
	result, err := s.Status(context.Background(), testAccess(claims))
	if err != nil || result.IDCaptured || result.Complete || facts.facts.IDCaptured {
		t.Fatal(result, err)
	}
	cfg.c.Features.IDCapture = false
	cfg.c.Onboarding.IDCapture = runtimeconfig.Disabled
	result, err = s.Status(context.Background(), testAccess(claims))
	if err != nil || result.IDCaptured || !result.Complete || facts.facts.IDCaptured {
		t.Fatal(result, err)
	}
	// An absent table is also safe if a caller supplies the read adapter explicitly.
	s.Captures = CaptureFactStore{}
	result, err = s.Status(context.Background(), testAccess(claims))
	if err != nil || result.IDCaptured || !result.Complete {
		t.Fatal(result, err)
	}
}
