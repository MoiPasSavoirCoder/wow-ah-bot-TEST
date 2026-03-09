import { Component } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { CommonModule } from '@angular/common';
import { BadgeModule } from 'primeng/badge';
import { RippleModule } from 'primeng/ripple';
import { TooltipModule } from 'primeng/tooltip';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, CommonModule, BadgeModule, RippleModule, TooltipModule],
  template: `
    <div class="layout-wrapper">

      <!-- ═══ Sidebar ═══ -->
      <div class="layout-sidebar">

        <!-- Logo -->
        <div class="sidebar-logo">
          <span class="logo-icon">⚔️</span>
          <div>
            <div class="logo-text">WoW AH Bot</div>
            <div class="logo-realm">Archimonde · EU · Retail</div>
          </div>
        </div>

        <!-- Navigation -->
        <nav class="sidebar-nav">
          <div class="nav-section-title">Navigation</div>

          <a routerLink="/dashboard" routerLinkActive="active" class="nav-item" pRipple>
            <i class="pi pi-home"></i>
            <span>Dashboard</span>
          </a>

          <a routerLink="/deals" routerLinkActive="active" class="nav-item" pRipple>
            <i class="pi pi-bolt"></i>
            <span>Opportunités</span>
          </a>

          <a routerLink="/portfolio" routerLinkActive="active" class="nav-item" pRipple>
            <i class="pi pi-wallet"></i>
            <span>Portfolio</span>
          </a>

          <div class="nav-section-title mt-3">Analyse</div>

          <a routerLink="/prices/0" routerLinkActive="active" class="nav-item" pRipple
             pTooltip="Recherchez un item depuis la page Deals" tooltipPosition="right">
            <i class="pi pi-chart-line"></i>
            <span>Historique des prix</span>
          </a>
        </nav>

        <!-- Footer status -->
        <div class="sidebar-footer">
          <span class="status-dot"></span>
          <span>Bot actif · scan /1h</span>
        </div>
      </div>

      <!-- ═══ Main ═══ -->
      <div class="layout-main">

        <!-- Topbar -->
        <div class="layout-topbar">
          <div class="flex align-items-center gap-2">
            <i class="pi pi-calendar" style="color: var(--accent-blue); font-size: 0.9rem;"></i>
            <span style="color: var(--text-color-secondary); font-size: 0.85rem;">
              {{ today | date:'EEEE dd MMMM yyyy' : '' : 'fr-FR' }}
            </span>
          </div>
          <div class="flex align-items-center gap-3">
            <span class="topbar-badge">
              <i class="pi pi-circle-fill"></i>
              Archimonde · EU
            </span>
          </div>
        </div>

        <!-- Page content -->
        <div class="layout-content">
          <router-outlet></router-outlet>
        </div>
      </div>

    </div>
  `,
})
export class AppComponent {
  today = new Date();
}

