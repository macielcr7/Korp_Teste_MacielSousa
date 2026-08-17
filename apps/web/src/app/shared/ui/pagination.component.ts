import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

@Component({
  selector: 'app-pagination',
  template: `
    @if (total() > 0) {
      <footer class="pagination">
        <span>
          Exibindo {{ firstVisible() }}-{{ lastVisible() }} de {{ total() }} {{ itemLabel() }}
        </span>
        <div>
          <button
            type="button"
            aria-label="Página anterior"
            [disabled]="normalizedPage() === 1"
            (click)="selectPage(normalizedPage() - 1)"
          >
            ‹
          </button>
          @for (page of pages(); track page) {
            <button
              type="button"
              [class.active]="page === normalizedPage()"
              [attr.aria-current]="page === normalizedPage() ? 'page' : null"
              (click)="selectPage(page)"
            >
              {{ page }}
            </button>
          }
          <button
            type="button"
            aria-label="Próxima página"
            [disabled]="normalizedPage() === totalPages()"
            (click)="selectPage(normalizedPage() + 1)"
          >
            ›
          </button>
        </div>
      </footer>
    }
  `,
  styles: `
    :host {
      display: block;
    }
    .pagination {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 1rem 0.25rem 0;
      color: var(--kf-muted);
      font-size: 0.75rem;
    }
    .pagination > div {
      display: flex;
      gap: 0.25rem;
    }
    button {
      min-width: 1.75rem;
      height: 1.75rem;
      padding: 0 0.4rem;
      border: 1px solid var(--kf-border);
      border-radius: 0.25rem;
      color: #475569;
      background: white;
      cursor: pointer;
    }
    button.active {
      color: white;
      border-color: var(--kf-blue);
      background: var(--kf-blue);
    }
    button:disabled {
      cursor: default;
      opacity: 0.4;
    }
    @media (max-width: 38rem) {
      .pagination {
        align-items: flex-start;
        flex-direction: column;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PaginationComponent {
  readonly total = input.required<number>();
  readonly currentPage = input.required<number>();
  readonly pageSize = input.required<number>();
  readonly itemLabel = input.required<string>();
  readonly pageChange = output<number>();

  protected readonly totalPages = computed(() =>
    Math.max(1, Math.ceil(Math.max(0, this.total()) / Math.max(1, this.pageSize()))),
  );
  protected readonly normalizedPage = computed(() =>
    Math.min(Math.max(this.currentPage(), 1), this.totalPages()),
  );
  protected readonly pages = computed(() => {
    const total = this.totalPages();
    const first = Math.min(Math.max(this.normalizedPage() - 1, 1), Math.max(total - 3, 1));
    return Array.from({ length: Math.min(total, 4) }, (_, index) => first + index);
  });
  protected readonly firstVisible = computed(() =>
    this.total() === 0 ? 0 : (this.normalizedPage() - 1) * Math.max(1, this.pageSize()) + 1,
  );
  protected readonly lastVisible = computed(() =>
    Math.min(this.normalizedPage() * Math.max(1, this.pageSize()), Math.max(0, this.total())),
  );

  protected selectPage(page: number): void {
    const nextPage = Math.min(Math.max(page, 1), this.totalPages());
    if (nextPage !== this.normalizedPage()) this.pageChange.emit(nextPage);
  }
}
