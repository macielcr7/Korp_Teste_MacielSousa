import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs';

import { AppIconComponent } from './shared/ui/app-icon.component';

@Component({
  selector: 'app-root',
  imports: [AppIconComponent, RouterLink, RouterLinkActive, RouterOutlet],
  templateUrl: './app.html',
  styleUrl: './app.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  private readonly router = inject(Router);
  private readonly url = signal(this.router.url);

  protected readonly section = computed(() => {
    const url = this.url();
    if (url === '/produtos/novo') return 'Novo Produto';
    if (url.startsWith('/produtos')) return 'Produtos';
    if (url === '/notas/nova') return 'Nova Nota';
    if (url.startsWith('/notas')) return 'Notas';
    return 'Início';
  });
  protected readonly isDashboard = computed(() => this.url() === '/');

  constructor() {
    this.router.events
      .pipe(
        filter((event): event is NavigationEnd => event instanceof NavigationEnd),
        takeUntilDestroyed(),
      )
      .subscribe((event) => this.url.set(event.urlAfterRedirects));
  }
}
