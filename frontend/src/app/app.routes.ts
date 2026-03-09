import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    redirectTo: 'dashboard',
    pathMatch: 'full',
  },
  {
    path: 'dashboard',
    loadComponent: () =>
      import('./pages/dashboard/dashboard.component').then(m => m.DashboardComponent),
  },
  {
    path: 'deals',
    loadComponent: () =>
      import('./pages/deals/deals.component').then(m => m.DealsComponent),
  },
  {
    path: 'portfolio',
    loadComponent: () =>
      import('./pages/portfolio/portfolio.component').then(m => m.PortfolioComponent),
  },
  {
    path: 'prices/:itemId',
    loadComponent: () =>
      import('./pages/price-history/price-history.component').then(m => m.PriceHistoryComponent),
  },
];
