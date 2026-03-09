package scanner

import (
	"log"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"wow-ah-bot/internal/database"
	"wow-ah-bot/internal/models"
	"wow-ah-bot/internal/services/blizzard"
)

// ScanResult is returned after a successful scan.
type ScanResult struct {
	ID             uint
	TotalAuctions  int
	TotalGoldVolume int64
	ScannedAt      time.Time
	UniqueItems    int
}

// Scan performs a full AH scan: fetch auctions, store entries, aggregate prices, cache new items.
func Scan() (*ScanResult, error) {
	log.Println("🔍 Starting AH scan...")
	db := database.DB

	// 1. Fetch auctions from Blizzard
	auctions, err := blizzard.GetAuctions()
	if err != nil {
		return nil, err
	}
	if len(auctions) == 0 {
		log.Println("⚠️  No auctions retrieved")
		return nil, nil
	}

	now := time.Now().UTC()

	// 2. Create snapshot
	snapshot := models.AuctionSnapshot{
		ScannedAt:     now,
		TotalAuctions: len(auctions),
	}
	if err := db.Create(&snapshot).Error; err != nil {
		return nil, err
	}

	// 3. Process auctions: group by item, batch-insert entries
	type itemAgg struct {
		prices   []int64
		totalQty int
		count    int
	}
	itemMap := make(map[int]*itemAgg, 2000)
	entries := make([]models.AuctionEntry, 0, len(auctions))
	var totalVolume int64

	for _, a := range auctions {
		itemID := a.Item.ID
		if itemID == 0 {
			continue
		}
		price := a.UnitPrice
		if price == 0 {
			price = a.Buyout
		}
		if price <= 0 {
			continue
		}

		entries = append(entries, models.AuctionEntry{
			SnapshotID: snapshot.ID,
			AuctionID:  a.ID,
			ItemID:     itemID,
			Quantity:   max(a.Quantity, 1),
			UnitPrice:  a.UnitPrice,
			Buyout:     a.Buyout,
			Bid:        a.Bid,
			TimeLeft:   a.TimeLeft,
		})

		agg, ok := itemMap[itemID]
		if !ok {
			agg = &itemAgg{}
			itemMap[itemID] = agg
		}
		qty := max(a.Quantity, 1)
		// Expand prices by quantity for proper median/avg
		for range qty {
			agg.prices = append(agg.prices, price)
		}
		agg.totalQty += qty
		agg.count++
		totalVolume += price * int64(qty)
	}

	// Batch-insert entries (in chunks of 1000)
	for i := 0; i < len(entries); i += 1000 {
		end := min(i+1000, len(entries))
		if err := db.CreateInBatches(entries[i:end], 1000).Error; err != nil {
			log.Printf("⚠️  Batch insert entries: %v", err)
		}
	}

	// Update snapshot volume
	db.Model(&snapshot).Update("total_gold_volume", totalVolume)

	// 4. Compute price aggregates per item
	priceHistories := make([]models.PriceHistory, 0, len(itemMap))
	for itemID, agg := range itemMap {
		if len(agg.prices) == 0 {
			continue
		}
		sort.Slice(agg.prices, func(i, j int) bool { return agg.prices[i] < agg.prices[j] })
		prices := agg.prices

		var sum int64
		for _, p := range prices {
			sum += p
		}

		priceHistories = append(priceHistories, models.PriceHistory{
			ItemID:        itemID,
			ScannedAt:     now,
			MinBuyout:     prices[0],
			AvgBuyout:     sum / int64(len(prices)),
			MedianBuyout:  prices[len(prices)/2],
			MaxBuyout:     prices[len(prices)-1],
			TotalQuantity: agg.totalQty,
			NumAuctions:   agg.count,
		})
	}

	for i := 0; i < len(priceHistories); i += 500 {
		end := min(i+500, len(priceHistories))
		if err := db.CreateInBatches(priceHistories[i:end], 500).Error; err != nil {
			log.Printf("⚠️  Batch insert price history: %v", err)
		}
	}

	log.Printf("✅ Scan complete: %d auctions, %d unique items", len(auctions), len(itemMap))

	// 5. Fetch details for unknown items (synchronous so names are ready for Analyze)
	allItemIDs := make([]int, 0, len(itemMap))
	for id := range itemMap {
		allItemIDs = append(allItemIDs, id)
	}
	updateUnknownItems(db, allItemIDs)

	return &ScanResult{
		ID:              snapshot.ID,
		TotalAuctions:   len(auctions),
		TotalGoldVolume: totalVolume,
		ScannedAt:       now,
		UniqueItems:     len(itemMap),
	}, nil
}

// updateUnknownItems fetches item details from Blizzard for items not yet cached.
// Uses 5 concurrent workers for speed. Blocks until all fetches complete.
func updateUnknownItems(db *gorm.DB, itemIDs []int) {
	// Find items already in cache
	var existingIDs []int
	db.Model(&models.Item{}).Where("id IN ?", itemIDs).Pluck("id", &existingIDs)
	existing := make(map[int]bool, len(existingIDs))
	for _, id := range existingIDs {
		existing[id] = true
	}

	// Also count items with empty names (need re-fetch)
	var emptyNameIDs []int
	db.Model(&models.Item{}).Where("id IN ? AND (name = '' OR name IS NULL)", itemIDs).Pluck("id", &emptyNameIDs)
	emptyNames := make(map[int]bool, len(emptyNameIDs))
	for _, id := range emptyNameIDs {
		emptyNames[id] = true
	}

	var toFetch []int
	for _, id := range itemIDs {
		if !existing[id] || emptyNames[id] {
			toFetch = append(toFetch, id)
		}
	}
	if len(toFetch) == 0 {
		return
	}
	if len(toFetch) > 100 {
		toFetch = toFetch[:100]
	}

	log.Printf("📦 Fetching details for %d items (concurrent)...", len(toFetch))

	// Concurrent fetch with 5 workers
	const workers = 5
	ch := make(chan int, len(toFetch))
	for _, id := range toFetch {
		ch <- id
	}
	close(ch)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for itemID := range ch {
				details, err := blizzard.GetItemWithDetails(itemID)
				if err != nil {
					continue
				}
				item := models.Item{
					ID:           details.ID,
					Name:         details.Name,
					Quality:      details.Quality,
					ItemClass:    details.ItemClass,
					ItemSubclass: details.ItemSubclass,
					Level:        details.Level,
					IconURL:      details.IconURL,
					VendorPrice:  details.VendorPrice,
				}
				// Upsert: update if exists with empty name, create otherwise
				db.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "id"}},
					DoUpdates: clause.AssignmentColumns([]string{"name", "quality", "item_class", "item_subclass", "level", "icon_url", "vendor_price", "updated_at"}),
				}).Create(&item)
			}
		}()
	}
	wg.Wait()
	log.Printf("✅ Item cache updated")
}
