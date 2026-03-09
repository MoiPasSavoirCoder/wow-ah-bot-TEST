import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { Subject, takeUntil, debounceTime, distinctUntilChanged } from 'rxjs';

// PrimeNG
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';
import { SkeletonModule } from 'primeng/skeleton';
import { TooltipModule } from 'primeng/tooltip';
import { DropdownModule } from 'primeng/dropdown';
import { PaginatorModule } from 'primeng/paginator';
import { BadgeModule } from 'primeng/badge';

import { ApiService } from '../../services/api.service';
import { AuctionHouseItem, AuctionHouseCategory, AuctionHouseResponse } from '../../models/interfaces';

@Component({
  selector: 'app-auction-house',
  standalone: true,
  imports: [
    CommonModule, FormsModule, RouterLink,
    InputTextModule, ButtonModule, SkeletonModule,
    TooltipModule, DropdownModule, PaginatorModule, BadgeModule,
  ],
  template: `
    <div class="ah-page">

      <!-- ═══ Header banner (WoW style) ═══ -->
      <div class="ah-banner">
        <div class="ah-banner-content">
          <div class="ah-title-row">
            <span class="ah-icon">🏛️</span>
            <div>
              <h1>Hôtel des Ventes</h1>
              <p class="ah-subtitle" *ngIf="response">
                {{ response.total | number }} objets disponibles — Scan du {{ response.scanned_at | date:'dd/MM/yyyy à HH:mm' }}
              </p>
            </div>
          </div>
          <div class="ah-search-bar">
            <span class="search-icon"><i class="pi pi-search"></i></span>
            <input
              type="text"
              class="ah-search-input"
              placeholder="Rechercher un objet..."
              [(ngModel)]="searchText"
              (ngModelChange)="onSearchChange($event)"
            />
            <button class="ah-search-clear" *ngIf="searchText" (click)="clearSearch()">
              <i class="pi pi-times"></i>
            </button>
          </div>
        </div>
      </div>

      <div class="ah-body">

        <!-- ═══ Sidebar: Categories ═══ -->
        <aside class="ah-sidebar">
          <div class="sidebar-title">Catégories</div>

          <div class="category-item"
               [class.active]="!selectedCategory"
               (click)="selectCategory(null)">
            <span class="cat-name">Toutes</span>
            <span class="cat-count" *ngIf="totalItemCount">{{ totalItemCount }}</span>
          </div>

          <!-- Quality filter -->
          <div class="sidebar-title mt-2">Qualité</div>
          <div class="quality-filters">
            <button *ngFor="let q of qualities"
                    class="quality-btn"
                    [class.active]="selectedQuality === q.value"
                    [style.--q-color]="q.color"
                    (click)="selectQuality(q.value)">
              {{ q.label }}
            </button>
          </div>

          <div class="sidebar-title mt-2">Catégories d'objets</div>
          <div *ngIf="loadingCategories" class="cat-skeleton">
            <p-skeleton *ngFor="let i of [1,2,3,4,5,6]" height="32px" borderRadius="6px" styleClass="mb-1"></p-skeleton>
          </div>

          <div *ngFor="let cat of categories" class="cat-group">
            <div class="category-item"
                 [class.active]="selectedCategory === cat.name && !selectedSubcategory"
                 (click)="selectCategory(cat.name)">
              <span class="cat-icon">{{ getCategoryIcon(cat.name) }}</span>
              <span class="cat-name">{{ cat.name }}</span>
              <span class="cat-count">{{ cat.total }}</span>
            </div>
            <div *ngIf="selectedCategory === cat.name" class="subcategories">
              <div *ngFor="let sub of cat.subcategories"
                   class="subcategory-item"
                   [class.active]="selectedSubcategory === sub.name"
                   (click)="selectSubcategory(sub.name); $event.stopPropagation()">
                <span class="sub-name">{{ sub.name }}</span>
                <span class="sub-count">{{ sub.count }}</span>
              </div>
            </div>
          </div>
        </aside>

        <!-- ═══ Main: Item Grid ═══ -->
        <main class="ah-main">

          <!-- Sort bar -->
          <div class="ah-sort-bar">
            <div class="sort-info">
              <span *ngIf="!loading">{{ response?.total || 0 }} résultats</span>
              <span *ngIf="selectedCategory" class="active-filter">
                {{ selectedCategory }}
                <span *ngIf="selectedSubcategory"> › {{ selectedSubcategory }}</span>
                <button class="clear-filter" (click)="selectCategory(null)"><i class="pi pi-times"></i></button>
              </span>
            </div>
            <div class="sort-controls">
              <span class="sort-label">Trier par :</span>
              <button *ngFor="let s of sortOptions"
                      class="sort-btn"
                      [class.active]="sortBy === s.value"
                      (click)="toggleSort(s.value)">
                {{ s.label }}
                <i *ngIf="sortBy === s.value" class="pi" [ngClass]="sortDir === 'asc' ? 'pi-arrow-up' : 'pi-arrow-down'"></i>
              </button>
            </div>
          </div>

          <!-- Loading skeleton -->
          <div *ngIf="loading" class="ah-grid">
            <div *ngFor="let i of skeletonArray" class="ah-item-card skeleton-card">
              <p-skeleton width="56px" height="56px" borderRadius="8px"></p-skeleton>
              <div class="skeleton-info">
                <p-skeleton width="140px" height="16px" borderRadius="4px"></p-skeleton>
                <p-skeleton width="80px" height="14px" borderRadius="4px" styleClass="mt-1"></p-skeleton>
              </div>
            </div>
          </div>

          <!-- Item cards -->
          <div *ngIf="!loading" class="ah-grid">
            <div *ngFor="let item of items; trackBy: trackByItemId"
                 class="ah-item-card"
                 [class]="'quality-border-' + (item.quality || 'common').toLowerCase()"
                 [routerLink]="['/prices', item.item_id]"
                 pTooltip="{{ item.item_class }} › {{ item.item_subclass }}"
                 tooltipPosition="top">

              <!-- Icon -->
              <div class="item-icon-wrapper" [class]="'quality-bg-' + (item.quality || 'common').toLowerCase()">
                <img *ngIf="item.icon_url" [src]="item.icon_url" [alt]="item.item_name" class="item-icon" (error)="onImgError($event)" />
                <div *ngIf="!item.icon_url" class="item-icon-placeholder">
                  <i class="pi pi-box"></i>
                </div>
                <span *ngIf="item.total_quantity > 1" class="item-qty">{{ item.total_quantity | number }}</span>
              </div>

              <!-- Info -->
              <div class="item-info">
                <span class="item-name" [class]="'quality-text-' + (item.quality || 'common').toLowerCase()">
                  {{ item.item_name }}
                </span>
                <span class="item-subclass">{{ item.item_subclass || item.item_class }}</span>
                <div class="item-price-row">
                  <span class="item-price">{{ item.min_buyout_gold }}</span>
                  <span class="item-price-label">min</span>
                </div>
                <div *ngIf="item.num_auctions > 1" class="item-auctions">
                  {{ item.num_auctions }} enchères
                </div>
              </div>

              <!-- Level badge -->
              <span *ngIf="item.level > 1" class="item-level">{{ item.level }}</span>

            </div>
          </div>

          <!-- Empty state -->
          <div *ngIf="!loading && items.length === 0" class="ah-empty">
            <i class="pi pi-search" style="font-size: 3rem; opacity: 0.3"></i>
            <p>Aucun objet trouvé</p>
            <span>Essayez de modifier vos filtres ou votre recherche.</span>
          </div>

          <!-- Pagination -->
          <div *ngIf="!loading && response && response.total_pages > 1" class="ah-pagination">
            <p-paginator
              [rows]="pageSize"
              [totalRecords]="response.total"
              [first]="(currentPage - 1) * pageSize"
              (onPageChange)="onPageChange($event)"
              [rowsPerPageOptions]="[50, 100, 200]"
            ></p-paginator>
          </div>
        </main>
      </div>
    </div>
  `,
  styles: [`
    /* ════════════════════════════════════════════════════════════════════
       WoW Auction House Page — Dark Fantasy Theme
       ════════════════════════════════════════════════════════════════════ */

    .ah-page {
      padding: 0;
      min-height: 100%;
    }

    /* ── Banner ── */
    .ah-banner {
      background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
      border-radius: 12px;
      padding: 1.5rem 2rem;
      margin-bottom: 1.25rem;
      border: 1px solid #2a2a4a;
      box-shadow: 0 4px 20px rgba(0,0,0,0.3);
    }
    .ah-banner-content {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 2rem;
      flex-wrap: wrap;
    }
    .ah-title-row {
      display: flex;
      align-items: center;
      gap: 0.75rem;
    }
    .ah-icon { font-size: 2.2rem; }
    .ah-banner h1 {
      font-size: 1.6rem;
      font-weight: 800;
      color: #e8d5b5;
      margin: 0;
      text-shadow: 0 2px 8px rgba(0,0,0,0.5);
      font-family: 'Georgia', serif;
    }
    .ah-subtitle {
      font-size: 0.82rem;
      color: #a8a8c0;
      margin: 0.2rem 0 0;
    }

    /* ── Search bar ── */
    .ah-search-bar {
      display: flex;
      align-items: center;
      background: rgba(255,255,255,0.08);
      border: 1px solid rgba(255,255,255,0.15);
      border-radius: 10px;
      padding: 0 1rem;
      min-width: 320px;
      transition: border-color 0.2s;
    }
    .ah-search-bar:focus-within {
      border-color: #e8d5b5;
      box-shadow: 0 0 0 2px rgba(232,213,181,0.2);
    }
    .search-icon { color: #a8a8c0; font-size: 0.9rem; margin-right: 0.5rem; }
    .ah-search-input {
      flex: 1;
      background: transparent;
      border: none;
      outline: none;
      color: #e8e8f0;
      font-size: 0.9rem;
      padding: 0.6rem 0;
    }
    .ah-search-input::placeholder { color: #6b6b8a; }
    .ah-search-clear {
      background: transparent;
      border: none;
      color: #6b6b8a;
      cursor: pointer;
      padding: 0.3rem;
      font-size: 0.8rem;
    }
    .ah-search-clear:hover { color: #e8e8f0; }

    /* ── Body layout ── */
    .ah-body {
      display: flex;
      gap: 1.25rem;
      min-height: calc(100vh - 260px);
    }

    /* ── Sidebar ── */
    .ah-sidebar {
      width: 240px;
      min-width: 240px;
      background: var(--surface-card);
      border: 1px solid var(--surface-border);
      border-radius: 10px;
      padding: 1rem;
      overflow-y: auto;
      max-height: calc(100vh - 260px);
    }
    .sidebar-title {
      font-size: 0.72rem;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--text-color-secondary);
      padding: 0 0.5rem;
      margin-bottom: 0.4rem;
    }
    .mt-2 { margin-top: 1rem; }

    .category-item {
      display: flex;
      align-items: center;
      gap: 0.4rem;
      padding: 0.4rem 0.6rem;
      border-radius: 6px;
      cursor: pointer;
      font-size: 0.84rem;
      color: var(--text-color);
      transition: background 0.15s;
    }
    .category-item:hover { background: var(--surface-hover); }
    .category-item.active {
      background: #2563eb20;
      color: #2563eb;
      font-weight: 700;
    }
    .cat-icon { font-size: 1rem; width: 20px; text-align: center; }
    .cat-name { flex: 1; }
    .cat-count {
      font-size: 0.72rem;
      color: var(--text-color-secondary);
      background: var(--surface-ground);
      padding: 0.1rem 0.4rem;
      border-radius: 99px;
    }
    .category-item.active .cat-count { background: #2563eb30; color: #2563eb; }

    .subcategories { padding-left: 1.6rem; }
    .subcategory-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.3rem 0.5rem;
      border-radius: 4px;
      cursor: pointer;
      font-size: 0.8rem;
      color: var(--text-color-secondary);
      transition: background 0.15s;
    }
    .subcategory-item:hover { background: var(--surface-hover); color: var(--text-color); }
    .subcategory-item.active { color: #2563eb; font-weight: 700; }
    .sub-count {
      font-size: 0.68rem;
      color: var(--text-color-secondary);
    }
    .cat-skeleton { display: flex; flex-direction: column; gap: 4px; }

    /* ── Quality filter buttons ── */
    .quality-filters { display: flex; flex-wrap: wrap; gap: 4px; padding: 0 0.3rem; }
    .quality-btn {
      border: 1px solid var(--surface-border);
      background: var(--surface-ground);
      border-radius: 6px;
      padding: 0.2rem 0.5rem;
      font-size: 0.72rem;
      font-weight: 600;
      cursor: pointer;
      color: var(--q-color, var(--text-color));
      transition: all 0.15s;
    }
    .quality-btn:hover { border-color: var(--q-color); }
    .quality-btn.active {
      background: var(--q-color);
      color: white;
      border-color: var(--q-color);
    }

    /* ── Main area ── */
    .ah-main { flex: 1; min-width: 0; }

    /* ── Sort bar ── */
    .ah-sort-bar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 0.75rem;
      flex-wrap: wrap;
      gap: 0.5rem;
    }
    .sort-info {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      font-size: 0.84rem;
      color: var(--text-color-secondary);
    }
    .active-filter {
      display: inline-flex;
      align-items: center;
      gap: 0.3rem;
      background: #2563eb15;
      color: #2563eb;
      padding: 0.2rem 0.6rem;
      border-radius: 6px;
      font-size: 0.78rem;
      font-weight: 600;
    }
    .clear-filter {
      background: transparent;
      border: none;
      color: #2563eb;
      cursor: pointer;
      font-size: 0.7rem;
      padding: 0.1rem;
    }
    .sort-controls { display: flex; align-items: center; gap: 0.3rem; }
    .sort-label { font-size: 0.78rem; color: var(--text-color-secondary); margin-right: 0.25rem; }
    .sort-btn {
      display: flex;
      align-items: center;
      gap: 0.25rem;
      border: 1px solid var(--surface-border);
      background: var(--surface-card);
      padding: 0.3rem 0.65rem;
      border-radius: 6px;
      font-size: 0.78rem;
      cursor: pointer;
      color: var(--text-color-secondary);
      transition: all 0.15s;
    }
    .sort-btn:hover { border-color: #2563eb; color: var(--text-color); }
    .sort-btn.active { background: #2563eb; color: white; border-color: #2563eb; }

    /* ── Item Grid ── */
    .ah-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
      gap: 0.6rem;
    }

    /* ── Item Card ── */
    .ah-item-card {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      background: var(--surface-card);
      border: 1px solid var(--surface-border);
      border-radius: 10px;
      padding: 0.65rem 0.85rem;
      cursor: pointer;
      transition: all 0.2s;
      position: relative;
      text-decoration: none;
    }
    .ah-item-card:hover {
      border-color: #2563eb;
      box-shadow: 0 2px 12px rgba(37,99,235,0.12);
      transform: translateY(-1px);
    }

    .skeleton-card {
      gap: 1rem;
      padding: 0.8rem 1rem;
    }
    .skeleton-info { flex: 1; }

    /* ── Quality borders ── */
    .quality-border-poor       { border-left: 3px solid #9d9d9d; }
    .quality-border-common     { border-left: 3px solid #ffffff; }
    .quality-border-uncommon   { border-left: 3px solid #1eff00; }
    .quality-border-rare       { border-left: 3px solid #0070dd; }
    .quality-border-epic       { border-left: 3px solid #a335ee; }
    .quality-border-legendary  { border-left: 3px solid #ff8000; }
    .quality-border-artifact   { border-left: 3px solid #e6cc80; }
    .quality-border-heirloom   { border-left: 3px solid #00ccff; }

    /* ── Quality text colors ── */
    .quality-text-poor       { color: #9d9d9d !important; }
    .quality-text-common     { color: var(--text-color) !important; }
    .quality-text-uncommon   { color: #1eff00 !important; }
    .quality-text-rare       { color: #0070dd !important; }
    .quality-text-epic       { color: #a335ee !important; }
    .quality-text-legendary  { color: #ff8000 !important; }
    .quality-text-artifact   { color: #e6cc80 !important; }
    .quality-text-heirloom   { color: #00ccff !important; }

    /* ── Quality icon backgrounds ── */
    .quality-bg-poor       { border-color: #9d9d9d40; }
    .quality-bg-common     { border-color: #ffffff20; }
    .quality-bg-uncommon   { border-color: #1eff0040; }
    .quality-bg-rare       { border-color: #0070dd40; }
    .quality-bg-epic       { border-color: #a335ee40; }
    .quality-bg-legendary  { border-color: #ff800050; }
    .quality-bg-artifact   { border-color: #e6cc8050; }
    .quality-bg-heirloom   { border-color: #00ccff40; }

    /* ── Icon ── */
    .item-icon-wrapper {
      width: 56px;
      height: 56px;
      min-width: 56px;
      border-radius: 8px;
      border: 2px solid var(--surface-border);
      background: linear-gradient(135deg, #1a1a2e, #16213e);
      display: flex;
      align-items: center;
      justify-content: center;
      position: relative;
      overflow: visible;
    }
    .item-icon {
      width: 100%;
      height: 100%;
      object-fit: cover;
      border-radius: 6px;
    }
    .item-icon-placeholder {
      color: #4a4a6a;
      font-size: 1.4rem;
    }
    .item-qty {
      position: absolute;
      bottom: -4px;
      right: -4px;
      background: #1a1a2e;
      color: #e8d5b5;
      font-size: 0.68rem;
      font-weight: 700;
      padding: 0.05rem 0.35rem;
      border-radius: 4px;
      border: 1px solid #2a2a4a;
    }

    /* ── Item info ── */
    .item-info {
      flex: 1;
      min-width: 0;
      display: flex;
      flex-direction: column;
      gap: 1px;
    }
    .item-name {
      font-size: 0.88rem;
      font-weight: 700;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .item-subclass {
      font-size: 0.72rem;
      color: var(--text-color-secondary);
    }
    .item-price-row {
      display: flex;
      align-items: baseline;
      gap: 0.3rem;
      margin-top: 2px;
    }
    .item-price {
      font-size: 0.88rem;
      font-weight: 800;
      color: #ffd700;
    }
    .item-price-label {
      font-size: 0.68rem;
      color: var(--text-color-secondary);
    }
    .item-auctions {
      font-size: 0.68rem;
      color: var(--text-color-secondary);
    }

    /* ── Level badge ── */
    .item-level {
      position: absolute;
      top: 6px;
      right: 8px;
      font-size: 0.68rem;
      color: var(--text-color-secondary);
      background: var(--surface-ground);
      padding: 0.05rem 0.35rem;
      border-radius: 4px;
      font-weight: 600;
    }

    /* ── Empty state ── */
    .ah-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 0.5rem;
      padding: 4rem 2rem;
      color: var(--text-color-secondary);
    }
    .ah-empty p { font-size: 1.1rem; font-weight: 600; margin: 0; }
    .ah-empty span { font-size: 0.85rem; }

    /* ── Pagination ── */
    .ah-pagination {
      margin-top: 1rem;
      display: flex;
      justify-content: center;
    }

    /* ── Responsive ── */
    @media (max-width: 900px) {
      .ah-body { flex-direction: column; }
      .ah-sidebar { width: 100%; min-width: auto; max-height: none; }
      .ah-banner-content { flex-direction: column; }
      .ah-search-bar { min-width: auto; width: 100%; }
    }

    .mt-1 { margin-top: 0.25rem; }
  `],
})
export class AuctionHouseComponent implements OnInit, OnDestroy {
  items: AuctionHouseItem[] = [];
  categories: AuctionHouseCategory[] = [];
  response: AuctionHouseResponse | null = null;

  loading = true;
  loadingCategories = true;

  searchText = '';
  selectedCategory: string | null = null;
  selectedSubcategory: string | null = null;
  selectedQuality: string | null = null;
  sortBy = 'name';
  sortDir = 'asc';
  currentPage = 1;
  pageSize = 50;

  private searchSubject = new Subject<string>();
  private destroy$ = new Subject<void>();

  skeletonArray = Array.from({ length: 12 });

  qualities = [
    { label: 'Pauvre',      value: 'Poor',      color: '#9d9d9d' },
    { label: 'Commun',     value: 'Common',    color: '#ffffff' },
    { label: 'Inhabituel', value: 'Uncommon',  color: '#1eff00' },
    { label: 'Rare',       value: 'Rare',      color: '#0070dd' },
    { label: 'Épique',     value: 'Epic',      color: '#a335ee' },
    { label: 'Légendaire', value: 'Legendary', color: '#ff8000' },
  ];

  sortOptions = [
    { label: 'Nom',      value: 'name' },
    { label: 'Prix',     value: 'price' },
    { label: 'Quantité', value: 'quantity' },
    { label: 'Niveau',   value: 'level' },
  ];

  get totalItemCount(): number {
    return this.categories.reduce((sum, c) => sum + c.total, 0);
  }

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.searchSubject.pipe(
      debounceTime(400),
      distinctUntilChanged(),
      takeUntil(this.destroy$),
    ).subscribe(() => {
      this.currentPage = 1;
      this.loadItems();
    });

    this.loadCategories();
    this.loadItems();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadCategories(): void {
    this.loadingCategories = true;
    this.api.getAuctionHouseCategories()
      .pipe(takeUntil(this.destroy$))
      .subscribe({
        next: cats => {
          this.categories = cats;
          this.loadingCategories = false;
        },
        error: () => { this.loadingCategories = false; },
      });
  }

  loadItems(): void {
    this.loading = true;
    this.api.getAuctionHouse({
      search: this.searchText || undefined,
      category: this.selectedCategory || undefined,
      subcategory: this.selectedSubcategory || undefined,
      quality: this.selectedQuality || undefined,
      sort: this.sortBy,
      dir: this.sortDir,
      page: this.currentPage,
      page_size: this.pageSize,
    }).pipe(takeUntil(this.destroy$))
      .subscribe({
        next: res => {
          this.response = res;
          this.items = res.items || [];
          this.loading = false;
        },
        error: () => { this.loading = false; },
      });
  }

  onSearchChange(value: string): void {
    this.searchSubject.next(value);
  }

  clearSearch(): void {
    this.searchText = '';
    this.currentPage = 1;
    this.loadItems();
  }

  selectCategory(cat: string | null): void {
    this.selectedCategory = cat;
    this.selectedSubcategory = null;
    this.currentPage = 1;
    this.loadItems();
  }

  selectSubcategory(sub: string): void {
    this.selectedSubcategory = sub;
    this.currentPage = 1;
    this.loadItems();
  }

  selectQuality(q: string): void {
    this.selectedQuality = this.selectedQuality === q ? null : q;
    this.currentPage = 1;
    this.loadItems();
  }

  toggleSort(field: string): void {
    if (this.sortBy === field) {
      this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      this.sortBy = field;
      this.sortDir = field === 'price' ? 'asc' : 'asc';
    }
    this.currentPage = 1;
    this.loadItems();
  }

  onPageChange(event: any): void {
    this.currentPage = Math.floor(event.first / event.rows) + 1;
    this.pageSize = event.rows;
    this.loadItems();
  }

  getCategoryIcon(name: string): string {
    const icons: Record<string, string> = {
      'Weapon':        '⚔️',
      'Armor':         '🛡️',
      'Consumable':    '🧪',
      'Container':     '📦',
      'Gem':           '💎',
      'Reagent':       '🌿',
      'Recipe':        '📜',
      'Miscellaneous': '🎲',
      'Item Enhancement': '✨',
      'Profession':    '🔧',
      'Tradeskill':    '🔧',
      'Trade Goods':   '🏪',
      'Tradegoods':    '🏪',
      'Quest':         '❗',
      'Glyph':         '🔮',
      'Battle Pets':   '🐾',
      'Companion Pets':'🐾',
      'Mount':         '🐎',
    };
    return icons[name] || '📋';
  }

  onImgError(event: Event): void {
    (event.target as HTMLImageElement).style.display = 'none';
  }

  trackByItemId(_: number, item: AuctionHouseItem): number {
    return item.item_id;
  }
}
