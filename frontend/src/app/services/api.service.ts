import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '@env/environment';
import {
  DashboardSummary,
  Deal,
  ItemScore,
  PortfolioEntry,
  GoldBalance,
  PriceHistoryList,
  InventoryItem,
  PnlSummary,
  WowItem,
} from '../models/interfaces';

@Injectable({
  providedIn: 'root',
})
export class ApiService {
  private baseUrl = environment.apiUrl;

  constructor(private http: HttpClient) {}

  // ─── Dashboard ───

  getDashboard(): Observable<DashboardSummary> {
    return this.http.get<DashboardSummary>(`${this.baseUrl}/dashboard`);
  }

  // ─── Deals ───

  getDeals(status?: string, dealType?: string, limit = 50): Observable<Deal[]> {
    let params = new HttpParams().set('limit', limit.toString());
    if (status) params = params.set('status', status);
    if (dealType) params = params.set('deal_type', dealType);
    return this.http.get<Deal[]>(`${this.baseUrl}/deals`, { params });
  }

  getScores(limit = 100): Observable<ItemScore[]> {
    return this.http.get<ItemScore[]>(`${this.baseUrl}/scores`, {
      params: { limit: limit.toString() },
    });
  }

  executeDeal(dealId: number): Observable<any> {
    return this.http.post(`${this.baseUrl}/deals/${dealId}/execute`, {});
  }

  skipDeal(dealId: number): Observable<any> {
    return this.http.post(`${this.baseUrl}/deals/${dealId}/skip`, {});
  }

  // ─── Portfolio ───

  getPortfolio(limit = 100): Observable<PortfolioEntry[]> {
    return this.http.get<PortfolioEntry[]>(`${this.baseUrl}/portfolio`, {
      params: { limit: limit.toString() },
    });
  }

  addPortfolioEntry(entry: {
    item_id: number;
    item_name?: string;
    action: string;
    quantity: number;
    price_per_unit: number;
    notes?: string;
  }): Observable<PortfolioEntry> {
    return this.http.post<PortfolioEntry>(`${this.baseUrl}/portfolio`, entry);
  }

  getInventory(): Observable<InventoryItem[]> {
    return this.http.get<InventoryItem[]>(`${this.baseUrl}/portfolio/inventory`);
  }

  getPnl(): Observable<PnlSummary> {
    return this.http.get<PnlSummary>(`${this.baseUrl}/portfolio/pnl`);
  }

  // ─── Gold History ───

  getGoldHistory(days = 30): Observable<GoldBalance[]> {
    return this.http.get<GoldBalance[]>(`${this.baseUrl}/gold-history`, {
      params: { days: days.toString() },
    });
  }

  // ─── Prices ───

  getPriceHistory(itemId: number, days = 7): Observable<PriceHistoryList> {
    return this.http.get<PriceHistoryList>(`${this.baseUrl}/prices/${itemId}`, {
      params: { days: days.toString() },
    });
  }

  // ─── Items ───

  searchItems(query: string, limit = 20): Observable<WowItem[]> {
    return this.http.get<WowItem[]>(`${this.baseUrl}/items/search`, {
      params: { q: query, limit: limit.toString() },
    });
  }

  // ─── Actions ───

  triggerScan(): Observable<any> {
    return this.http.post(`${this.baseUrl}/scan`, {});
  }

  triggerAnalysis(): Observable<any> {
    return this.http.post(`${this.baseUrl}/analyze`, {});
  }

  triggerRefresh(): Observable<{ message: string; total_auctions: number; unique_items: number; deals_count: number; scanned_at: string }> {
    return this.http.post<any>(`${this.baseUrl}/refresh`, {});
  }
}
