import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./features/dashboard/dashboard.component').then(
        (module) => module.DashboardComponent,
      ),
    title: 'Visão operacional | Korp Flow',
  },
  {
    path: 'produtos',
    loadComponent: () =>
      import('./features/products/presentation/product-list/product-list.component').then(
        (module) => module.ProductListComponent,
      ),
    title: 'Produtos | Korp',
  },
  {
    path: 'produtos/novo',
    loadComponent: () =>
      import('./features/products/presentation/product-form/product-form.component').then(
        (module) => module.ProductFormComponent,
      ),
    title: 'Novo produto | Korp',
  },
  {
    path: 'notas',
    loadComponent: () =>
      import('./features/invoices/presentation/invoice-list/invoice-list.component').then(
        (module) => module.InvoiceListComponent,
      ),
    title: 'Notas | Korp',
  },
  {
    path: 'notas/nova',
    loadComponent: () =>
      import('./features/invoices/presentation/invoice-form/invoice-form.component').then(
        (module) => module.InvoiceFormComponent,
      ),
    title: 'Nova nota | Korp',
  },
  {
    path: 'notas/:id/impressao',
    loadComponent: () =>
      import('./features/invoices/presentation/invoice-print/invoice-print.component').then(
        (module) => module.InvoicePrintComponent,
      ),
    title: 'Imprimir nota | Korp',
  },
  {
    path: 'notas/:id',
    loadComponent: () =>
      import('./features/invoices/presentation/invoice-detail/invoice-detail.component').then(
        (module) => module.InvoiceDetailComponent,
      ),
    title: 'Detalhes da nota | Korp',
  },
  {
    path: '**',
    loadComponent: () =>
      import('./shared/ui/not-found.component').then((module) => module.NotFoundComponent),
    title: 'Página não encontrada | Korp',
  },
];
