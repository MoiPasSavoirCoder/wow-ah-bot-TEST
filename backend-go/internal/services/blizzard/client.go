package blizzard

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"wow-ah-bot/internal/config"
)

// ════════════════════════════════════════
// OAuth2 Token Manager
// ════════════════════════════════════════

type tokenManager struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

var auth = &tokenManager{}

func (tm *tokenManager) getToken() (string, error) {
	tm.mu.RLock()
	if tm.token != "" && time.Now().Before(tm.expiresAt) {
		t := tm.token
		tm.mu.RUnlock()
		return t, nil
	}
	tm.mu.RUnlock()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if tm.token != "" && time.Now().Before(tm.expiresAt) {
		return tm.token, nil
	}

	cfg := config.Cfg
	data := url.Values{"grant_type": {"client_credentials"}}

	req, err := http.NewRequest("POST", cfg.BlizzardTokenURL(), strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.SetBasicAuth(cfg.BlizzardClientID, cfg.BlizzardClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}

	tm.token = result.AccessToken
	tm.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	log.Println("🔑 Blizzard OAuth2 token refreshed")
	return tm.token, nil
}

// ════════════════════════════════════════
// HTTP Client
// ════════════════════════════════════════

var httpClient = &http.Client{Timeout: 30 * time.Second}

func doGet(apiURL, namespace string) ([]byte, error) {
	token, err := auth.getToken()
	if err != nil {
		return nil, err
	}

	cfg := config.Cfg
	parsedURL, _ := url.Parse(apiURL)
	q := parsedURL.Query()
	q.Set("locale", cfg.BlizzardLocale)
	parsedURL.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Battlenet-Namespace", namespace+"-"+cfg.BlizzardRegion)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s → HTTP %d: %s", apiURL, resp.StatusCode, body)
	}

	return io.ReadAll(resp.Body)
}

// ════════════════════════════════════════
// Public API Methods
// ════════════════════════════════════════

// Auction from Blizzard API.
type Auction struct {
	ID       int64 `json:"id"`
	Item     struct {
		ID int `json:"id"`
	} `json:"item"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	Buyout    int64  `json:"buyout"`
	Bid       int64  `json:"bid"`
	TimeLeft  string `json:"time_left"`
}

// GetAuctions fetches all auctions for the connected realm.
func GetAuctions() ([]Auction, error) {
	cfg := config.Cfg
	body, err := doGet(cfg.BlizzardAHURL(), "dynamic")
	if err != nil {
		return nil, err
	}

	var result struct {
		Auctions []Auction `json:"auctions"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode auctions: %w", err)
	}

	log.Printf("📦 Retrieved %d auctions from Blizzard API", len(result.Auctions))
	return result.Auctions, nil
}

// ItemDetails from Blizzard API.
type ItemDetails struct {
	ID          int
	Name        string
	Quality     string
	ItemClass   string
	ItemSubclass string
	Level       int
	IconURL     string
	VendorPrice int64
}

// GetItemWithDetails fetches item data + media icon.
func GetItemWithDetails(itemID int) (*ItemDetails, error) {
	cfg := config.Cfg
	base := cfg.BlizzardAPIBaseURL()

	// Fetch item data
	itemURL := fmt.Sprintf("%s/data/wow/item/%d", base, itemID)
	body, err := doGet(itemURL, "static")
	if err != nil {
		return nil, err
	}

	var raw struct {
		Name     string `json:"name"`
		Quality  struct {
			Type string `json:"type"`
		} `json:"quality"`
		ItemClass struct {
			Name string `json:"name"`
		} `json:"item_class"`
		ItemSubclass struct {
			Name string `json:"name"`
		} `json:"item_subclass"`
		Level     int   `json:"level"`
		SellPrice int64 `json:"sell_price"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	name := raw.Name
	if name == "" {
		name = fmt.Sprintf("Item #%d", itemID)
	}

	// Fetch media (icon URL)
	iconURL := ""
	mediaURL := fmt.Sprintf("%s/data/wow/media/item/%d", base, itemID)
	mediaBody, err := doGet(mediaURL, "static")
	if err == nil {
		var media struct {
			Assets []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"assets"`
		}
		if json.Unmarshal(mediaBody, &media) == nil {
			for _, a := range media.Assets {
				if a.Key == "icon" {
					iconURL = a.Value
					break
				}
			}
		}
	}

	return &ItemDetails{
		ID:           itemID,
		Name:         name,
		Quality:      raw.Quality.Type,
		ItemClass:    raw.ItemClass.Name,
		ItemSubclass: raw.ItemSubclass.Name,
		Level:        raw.Level,
		IconURL:      iconURL,
		VendorPrice:  raw.SellPrice,
	}, nil
}
