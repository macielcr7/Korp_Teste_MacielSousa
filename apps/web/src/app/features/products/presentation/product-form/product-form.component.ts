import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  ValidationErrors,
  Validators,
} from '@angular/forms';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { Router, RouterLink } from '@angular/router';

import { ErrorBannerComponent } from '../../../../shared/ui/error-banner.component';
import { ProductFormFacade } from '../../application/product-form.facade';

export const trimmedRequiredValidator = (control: AbstractControl): ValidationErrors | null =>
  typeof control.value === 'string' && control.value.trim().length > 0 ? null : { required: true };

@Component({
  imports: [ErrorBannerComponent, MatProgressSpinnerModule, ReactiveFormsModule, RouterLink],
  templateUrl: './product-form.component.html',
  styleUrl: './product-form.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [ProductFormFacade],
})
export class ProductFormComponent {
  private readonly facade = inject(ProductFormFacade);
  private readonly router = inject(Router);

  protected readonly submitting = this.facade.submitting;
  protected readonly error = this.facade.error;
  protected readonly form = new FormGroup({
    code: new FormControl('', {
      nonNullable: true,
      validators: [
        Validators.required,
        Validators.maxLength(64),
        Validators.pattern(/^[A-Za-z0-9._-]+$/),
      ],
    }),
    description: new FormControl('', {
      nonNullable: true,
      validators: [trimmedRequiredValidator, Validators.maxLength(255)],
    }),
    balance: new FormControl(0, {
      nonNullable: true,
      validators: [
        Validators.required,
        Validators.min(0),
        Validators.max(Number.MAX_SAFE_INTEGER),
        Validators.pattern(/^\d+$/),
      ],
    }),
  });

  protected submit(): void {
    if (this.form.invalid || this.submitting()) {
      this.form.markAllAsTouched();
      return;
    }

    const value = this.form.getRawValue();

    this.facade
      .create({
        code: value.code.trim().toUpperCase(),
        description: value.description.trim(),
        balance: value.balance,
      })
      .subscribe(() => void this.router.navigate(['/produtos']));
  }
}
