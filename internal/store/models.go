package store

import "time"

type User struct {
	ID                 int64      `json:"id"`
	TelegramID         int64      `json:"telegram_id"`
	Username           string     `json:"username"`
	FirstName          string     `json:"first_name"`
	PhotoURL           string     `json:"photo_url"`
	Language           string     `json:"language"`
	IsAdmin            bool       `json:"is_admin"`
	IsBlocked          bool       `json:"is_blocked"`
	TrialUsed          bool       `json:"trial_used"`
	ReferralRewarded   bool       `json:"referral_rewarded"`
	ReferralCode       string     `json:"referral_code"`
	ReferredByID       *int64     `json:"referred_by_id"`
	RemnawaveUserID    *int64     `json:"remnawave_user_id"`
	RemnawaveUserUUID  string     `json:"remnawave_user_uuid"`
	RemnawaveUsername  string     `json:"remnawave_username"`
	SubscriptionURL    string     `json:"subscription_url"`
	SubscriptionStatus string     `json:"subscription_status"`
	ExpiresAt          *time.Time `json:"expires_at"`
	TrafficLimitBytes  int64      `json:"traffic_limit_bytes"`
	TrafficUsedBytes   int64      `json:"traffic_used_bytes"`
	DeviceLimit        int        `json:"device_limit"`
	CreatedAt          time.Time  `json:"created_at"`
	LastSeenAt         time.Time  `json:"last_seen_at"`
}

type Tariff struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	PriceKopecks      int64     `json:"price_kopecks"`
	Days              int       `json:"days"`
	TrafficGB         int       `json:"traffic_gb"`
	DeviceLimit       int       `json:"device_limit"`
	InternalSquads    []string  `json:"internal_squads"`
	ExternalSquadUUID string    `json:"external_squad_uuid"`
	Active            bool      `json:"active"`
	Pinned            bool      `json:"pinned"`
	Position          int       `json:"position"`
	CreatedAt         time.Time `json:"created_at"`
}

type Payment struct {
	ID                string         `json:"id"`
	UserID            int64          `json:"user_id"`
	TariffID          string         `json:"tariff_id"`
	Provider          string         `json:"provider"`
	ProviderPaymentID string         `json:"provider_payment_id"`
	Status            string         `json:"status"`
	AmountKopecks     int64          `json:"amount_kopecks"`
	PromoCode         string         `json:"promo_code"`
	Snapshot          map[string]any `json:"snapshot"`
	Entitlement       map[string]any `json:"entitlement"`
	CreatedAt         time.Time      `json:"created_at"`
	PaidAt            *time.Time     `json:"paid_at"`
}

type Promo struct {
	ID              string     `json:"id"`
	Code            string     `json:"code"`
	DiscountPercent int        `json:"discount_percent"`
	MaxUses         int        `json:"max_uses"`
	Uses            int        `json:"uses"`
	Active          bool       `json:"active"`
	ExpiresAt       *time.Time `json:"expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type TicketMessage struct {
	ID           string    `json:"id"`
	TicketID     string    `json:"ticket_id"`
	SenderUserID *int64    `json:"sender_user_id"`
	IsAdmin      bool      `json:"is_admin"`
	Text         string    `json:"text"`
	CreatedAt    time.Time `json:"created_at"`
}

type Ticket struct {
	ID        string          `json:"id"`
	UserID    int64           `json:"user_id"`
	Subject   string          `json:"subject"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	User      *User           `json:"user,omitempty"`
	Messages  []TicketMessage `json:"messages"`
}

type Diagnostic struct {
	ID        string         `json:"id"`
	Source    string         `json:"source"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	Resolved  bool           `json:"resolved"`
	CreatedAt time.Time      `json:"created_at"`
}

type Broadcast struct {
	ID              string              `json:"id"`
	AdminUserID     int64               `json:"admin_user_id"`
	Text            string              `json:"text"`
	SourceChatID    int64               `json:"source_chat_id"`
	SourceMessageID int                 `json:"source_message_id"`
	Buttons         []map[string]string `json:"buttons"`
	Status          string              `json:"status"`
	SentCount       int                 `json:"sent_count"`
	FailedCount     int                 `json:"failed_count"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

type BroadcastDelivery struct {
	ChatID   int64
	Attempts int
}

type TelegramUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	FirstName    string `json:"first_name"`
	PhotoURL     string `json:"photo_url"`
	LanguageCode string `json:"language_code"`
}
