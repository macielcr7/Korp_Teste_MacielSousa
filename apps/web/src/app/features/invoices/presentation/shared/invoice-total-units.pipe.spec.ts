import { InvoiceTotalUnitsPipe } from './invoice-total-units.pipe';

describe('InvoiceTotalUnitsPipe', () => {
  it('sums item quantities without mutating the collection', () => {
    const items = Object.freeze([Object.freeze({ quantity: 2 }), Object.freeze({ quantity: 3 })]);

    expect(new InvoiceTotalUnitsPipe().transform(items)).toBe(5);
  });

  it('returns zero for an absent collection', () => {
    expect(new InvoiceTotalUnitsPipe().transform(undefined)).toBe(0);
  });
});
