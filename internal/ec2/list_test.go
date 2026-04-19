package ec2

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockPager implements instancePager for tests.
type mockPager struct {
	pages []*awsec2.DescribeInstancesOutput
	idx   int
	err   error
}

func (m *mockPager) HasMorePages() bool { return m.idx < len(m.pages) }
func (m *mockPager) NextPage(_ context.Context, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	p := m.pages[m.idx]
	m.idx++
	return p, nil
}

func ptr[T any](v T) *T { return &v }

// --- toInstance tests ---

func TestToInstance_AllFields(t *testing.T) {
	in := types.Instance{
		InstanceId:      ptr("i-abc123"),
		InstanceType:    types.InstanceTypeT3Micro,
		ImageId:         ptr("ami-999"),
		PrivateIpAddress: ptr("10.0.0.1"),
		PlatformDetails: ptr("Linux/UNIX"),
		State:           &types.InstanceState{Name: types.InstanceStateNameRunning},
		Tags:            []types.Tag{{Key: ptr("Name"), Value: ptr("web-server")}},
	}
	got := toInstance(in)
	if got.ID != "i-abc123" {
		t.Errorf("ID: got %q, want %q", got.ID, "i-abc123")
	}
	if got.Type != "t3.micro" {
		t.Errorf("Type: got %q, want %q", got.Type, "t3.micro")
	}
	if got.AMIID != "ami-999" {
		t.Errorf("AMIID: got %q, want %q", got.AMIID, "ami-999")
	}
	if got.PrivateIP != "10.0.0.1" {
		t.Errorf("PrivateIP: got %q, want %q", got.PrivateIP, "10.0.0.1")
	}
	if got.Platform != "Linux/UNIX" {
		t.Errorf("Platform: got %q, want %q", got.Platform, "Linux/UNIX")
	}
	if got.State != "running" {
		t.Errorf("State: got %q, want %q", got.State, "running")
	}
	if got.Name != "web-server" {
		t.Errorf("Name: got %q, want %q", got.Name, "web-server")
	}
}

func TestToInstance_NoNameTag(t *testing.T) {
	in := types.Instance{
		InstanceId: ptr("i-xyz"),
		Tags:       []types.Tag{{Key: ptr("env"), Value: ptr("prod")}},
	}
	got := toInstance(in)
	if got.Name != "" {
		t.Errorf("Name: got %q, want empty", got.Name)
	}
}

func TestToInstance_NameAmongMultipleTags(t *testing.T) {
	in := types.Instance{
		InstanceId: ptr("i-xyz"),
		Tags: []types.Tag{
			{Key: ptr("env"), Value: ptr("prod")},
			{Key: ptr("Name"), Value: ptr("api")},
			{Key: ptr("team"), Value: ptr("platform")},
		},
	}
	got := toInstance(in)
	if got.Name != "api" {
		t.Errorf("Name: got %q, want %q", got.Name, "api")
	}
}

func TestToInstance_NilPointerFields(t *testing.T) {
	in := types.Instance{
		InstanceId:      ptr("i-nil"),
		PrivateIpAddress: nil,
		PlatformDetails: nil,
		State:           nil,
	}
	got := toInstance(in)
	if got.PrivateIP != "" {
		t.Errorf("PrivateIP: got %q, want empty", got.PrivateIP)
	}
	if got.Platform != "" {
		t.Errorf("Platform: got %q, want empty", got.Platform)
	}
	if got.State != "" {
		t.Errorf("State: got %q, want empty", got.State)
	}
}

// --- listFromPager tests ---

func makeReservation(ids ...string) types.Reservation {
	var insts []types.Instance
	for _, id := range ids {
		insts = append(insts, types.Instance{InstanceId: awssdk.String(id)})
	}
	return types.Reservation{Instances: insts}
}

func TestListFromPager_SinglePage(t *testing.T) {
	pager := &mockPager{
		pages: []*awsec2.DescribeInstancesOutput{
			{Reservations: []types.Reservation{makeReservation("i-1", "i-2")}},
		},
	}
	got, err := listFromPager(context.Background(), pager)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got[0].ID != "i-1" || got[1].ID != "i-2" {
		t.Errorf("IDs: got %v", []string{got[0].ID, got[1].ID})
	}
}

func TestListFromPager_MultiplePages(t *testing.T) {
	pager := &mockPager{
		pages: []*awsec2.DescribeInstancesOutput{
			{Reservations: []types.Reservation{makeReservation("i-a")}},
			{Reservations: []types.Reservation{makeReservation("i-b")}},
			{Reservations: []types.Reservation{makeReservation("i-c")}},
		},
	}
	got, err := listFromPager(context.Background(), pager)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3", len(got))
	}
	ids := []string{got[0].ID, got[1].ID, got[2].ID}
	want := []string{"i-a", "i-b", "i-c"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestListFromPager_Empty(t *testing.T) {
	pager := &mockPager{pages: []*awsec2.DescribeInstancesOutput{}}
	got, err := listFromPager(context.Background(), pager)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len: got %d, want 0", len(got))
	}
}

func TestListFromPager_PaginatorError(t *testing.T) {
	sentinel := errors.New("api error")
	pager := &mockPager{
		pages: []*awsec2.DescribeInstancesOutput{
			{Reservations: []types.Reservation{makeReservation("i-1")}},
		},
		err: sentinel,
	}
	// HasMorePages returns true (idx=0, len=1), but NextPage returns error
	got, err := listFromPager(context.Background(), pager)
	if !errors.Is(err, sentinel) {
		t.Errorf("err: got %v, want %v", err, sentinel)
	}
	if got != nil {
		t.Errorf("expected nil slice on error, got %v", got)
	}
}
