import { FormControl } from '@angular/forms';

import { trimmedRequiredValidator } from './product-form.component';

describe('trimmedRequiredValidator', () => {
  it('rejects descriptions containing only whitespace', () => {
    const control = new FormControl('   ');

    expect(trimmedRequiredValidator(control)).toEqual({ required: true });
  });

  it('accepts descriptions with visible content', () => {
    const control = new FormControl('  Produto válido  ');

    expect(trimmedRequiredValidator(control)).toBeNull();
  });
});
