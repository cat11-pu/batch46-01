package task

import (
	"errors"
	"time"
)

// Sentinel errors reported by CraftingQueue. A duplicated order id, a missing
// material and a queue-cap rejection must be distinguishable.
var (
	ErrUnknownAccount       = errors.New("craft: unknown account")
	ErrUnknownCharacter     = errors.New("craft: unknown character")
	ErrCrossAccount         = errors.New("craft: character or order belongs to another account")
	ErrUnknownRecipe        = errors.New("craft: unknown recipe")
	ErrDuplicateRecipe      = errors.New("craft: recipe already registered")
	ErrInvalidRecipe        = errors.New("craft: invalid recipe")
	ErrDuplicateOrder       = errors.New("craft: duplicate order id")
	ErrInvalidOrderID       = errors.New("craft: invalid order id")
	ErrInvalidPriority      = errors.New("craft: priority out of range")
	ErrQueueFull            = errors.New("craft: character queue is full")
	ErrAccountLimit         = errors.New("craft: account order limit reached")
	ErrInsufficientMaterials = errors.New("craft: insufficient materials")
	ErrOrderNotFound        = errors.New("craft: order not found")
	ErrOrderNotPending      = errors.New("craft: order is not pending")
	ErrNotHead              = errors.New("craft: order is not the queue head")
	ErrQueueEmpty           = errors.New("craft: queue is empty")
	ErrInvalidQty           = errors.New("craft: invalid quantity")
)

var errNotImplemented = errors.New("not implemented")

// Config bounds the crafting service.
type Config struct {
	// SlotsPerCharacter is how many pending orders one character may hold.
	SlotsPerCharacter int
	// MaxOrdersPerAccount is the pending order budget of a whole account.
	MaxOrdersPerAccount int
	// ReservationTTL is how long materials stay reserved for an order.
	ReservationTTL time.Duration
	// MaxPriority is the highest accepted priority value.
	MaxPriority int
}

// Material is one stack of an item.
type Material struct {
	ItemID string
	Qty    int
}

// Recipe consumes Materials and produces Output.
type Recipe struct {
	ID        string
	Materials []Material
	Output    []Material
}

// OrderState is the lifecycle of one crafting order.
type OrderState int

const (
	// OrderPending still holds its material reservation.
	OrderPending OrderState = iota
	// OrderCompleted consumed its reservation and credited its output.
	OrderCompleted
	// OrderCancelled refunded its reservation.
	OrderCancelled
	// OrderExpired refunded its reservation after the TTL elapsed.
	OrderExpired
)

// Order is one queued craft.
type Order struct {
	OrderID     string
	AccountID   string
	CharacterID string
	RecipeID    string
	Priority    int
	// Seq is the enqueue sequence number; it breaks ties deterministically.
	Seq        int
	EnqueuedAt time.Time
	State      OrderState
}

// EnqueueRequest asks to queue one recipe for a character.
type EnqueueRequest struct {
	OrderID     string
	AccountID   string
	CharacterID string
	RecipeID    string
	Priority    int
}

// CraftingQueue reserves materials per account and serves per-character
// crafting queues.
type CraftingQueue struct {
	cfg Config
}

// NewCraftingQueue builds an empty crafting service.
func NewCraftingQueue(cfg Config) *CraftingQueue {
	return &CraftingQueue{cfg: cfg}
}

// NewCraftingQueueFromSnapshot restores inventories, reservations, recipes,
// orders and the sequence counter from a Snapshot payload.
func NewCraftingQueueFromSnapshot(cfg Config, data []byte) (*CraftingQueue, error) {
	return nil, errNotImplemented
}

// OpenAccount creates an account wallet of materials. Repeating it is a no-op.
func (q *CraftingQueue) OpenAccount(accountID string) error {
	return errNotImplemented
}

// OpenCharacter registers a character under an account. Character ids are
// globally unique: opening somebody else's character returns ErrCrossAccount.
func (q *CraftingQueue) OpenCharacter(accountID, characterID string) error {
	return errNotImplemented
}

// Grant adds materials to an account inventory. Quantity must be positive.
func (q *CraftingQueue) Grant(accountID, itemID string, qty int) error {
	return errNotImplemented
}

// AvailableQty returns the unreserved amount of an item.
func (q *CraftingQueue) AvailableQty(accountID, itemID string) int {
	return 0
}

// ReservedQty returns the amount of an item held by pending orders.
func (q *CraftingQueue) ReservedQty(accountID, itemID string) int {
	return 0
}

// RegisterRecipe validates and stores a recipe. Duplicate ids are rejected.
func (q *CraftingQueue) RegisterRecipe(r Recipe) error {
	return errNotImplemented
}

// Enqueue reserves every material of the recipe atomically and appends the
// order to the character queue. If any material is short nothing is reserved.
// Duplicate order ids are rejected even when the first order was cancelled.
func (q *CraftingQueue) Enqueue(req EnqueueRequest, now time.Time) (Order, error) {
	return Order{}, errNotImplemented
}

// NextOrder returns the head of the account queue: highest priority first,
// then earliest enqueue time, then lowest sequence number.
func (q *CraftingQueue) NextOrder(accountID string) (Order, error) {
	return Order{}, errNotImplemented
}

// Complete finishes the head order, consumes its reservation and credits the
// recipe output. Orders that are not the head, or are no longer pending, fail.
func (q *CraftingQueue) Complete(accountID, orderID string, now time.Time) ([]Material, error) {
	return nil, errNotImplemented
}

// Cancel refunds a pending order of any position and returns the refunded
// materials in recipe order.
func (q *CraftingQueue) Cancel(accountID, orderID string, now time.Time) ([]Material, error) {
	return nil, errNotImplemented
}

// PendingOrders returns the pending orders of an account in the order they
// will be served.
func (q *CraftingQueue) PendingOrders(accountID string) []Order {
	return nil
}

// Expire refunds every pending order whose reservation TTL elapsed at now and
// returns how many orders were expired.
func (q *CraftingQueue) Expire(now time.Time) int {
	return 0
}

// Snapshot serialises inventories, reservations, orders and the sequence
// counter so a restart keeps the same queue.
func (q *CraftingQueue) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
