import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';

import { InvoiceStatus } from '../../domain/invoice';

@Component({
  selector: 'app-invoice-status-badge',
  host: {
    class: 'status-chip',
    '[class.closed]': "status() === 'CLOSED' && !processing()",
    '[class.processing]': 'processing()',
  },
  template: `{{ label() }}`,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class InvoiceStatusBadgeComponent {
  readonly status = input.required<InvoiceStatus>();
  readonly processing = input(false);
  protected readonly label = computed(() =>
    this.processing() ? 'Processando' : this.status() === 'CLOSED' ? 'Fechada' : 'Aberta',
  );
}
