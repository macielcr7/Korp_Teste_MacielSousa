import { registerLocaleData } from '@angular/common';
import localePt from '@angular/common/locales/pt';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { ApplicationConfig, LOCALE_ID, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideRouter, withInMemoryScrolling } from '@angular/router';

import { apiErrorInterceptor } from './core/errors/api-error.interceptor';
import { IdempotencyStore } from './core/application/idempotency-store';
import { SessionIdempotencyStore } from './core/infrastructure/session-idempotency-store';
import { BillingGateway } from './features/invoices/application/billing.gateway';
import { BillingApiService } from './features/invoices/infrastructure/billing-api.service';
import { ProductGateway } from './features/products/application/product.gateway';
import { ProductApiService } from './features/products/infrastructure/product-api.service';
import { routes } from './app.routes';

registerLocaleData(localePt);

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    { provide: LOCALE_ID, useValue: 'pt-BR' },
    { provide: BillingGateway, useExisting: BillingApiService },
    { provide: ProductGateway, useExisting: ProductApiService },
    { provide: IdempotencyStore, useExisting: SessionIdempotencyStore },
    provideHttpClient(withInterceptors([apiErrorInterceptor])),
    provideRouter(routes, withInMemoryScrolling({ scrollPositionRestoration: 'top' })),
  ],
};
