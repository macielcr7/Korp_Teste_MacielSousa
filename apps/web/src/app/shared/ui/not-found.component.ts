import { ChangeDetectionStrategy, Component } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

@Component({
  imports: [MatButtonModule, RouterLink],
  template: `
    <section class="empty-state">
      <p class="eyebrow">Erro 404</p>
      <h1>Página não encontrada</h1>
      <p>O endereço informado não existe ou foi movido.</p>
      <a mat-flat-button routerLink="/produtos">Voltar para produtos</a>
    </section>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class NotFoundComponent {}
