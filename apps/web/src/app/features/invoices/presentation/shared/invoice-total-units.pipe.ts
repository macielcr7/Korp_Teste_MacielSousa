import { Pipe, PipeTransform } from '@angular/core';

@Pipe({ name: 'invoiceTotalUnits' })
export class InvoiceTotalUnitsPipe implements PipeTransform {
  transform(items: readonly { readonly quantity: number }[] | null | undefined): number {
    return items?.reduce((total, item) => total + item.quantity, 0) ?? 0;
  }
}
