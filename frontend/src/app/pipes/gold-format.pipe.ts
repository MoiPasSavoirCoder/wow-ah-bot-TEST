/**
 * Utility pipe to convert copper amounts to gold display string.
 */
import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'goldFormat',
  standalone: true,
})
export class GoldFormatPipe implements PipeTransform {
  transform(copperAmount: number | null | undefined): string {
    if (!copperAmount) return '0g';

    const gold = Math.floor(copperAmount / 10000);
    const silver = Math.floor((copperAmount % 10000) / 100);
    const copper = copperAmount % 100;

    const parts: string[] = [];
    if (gold) parts.push(`${gold.toLocaleString('fr-FR')}g`);
    if (silver) parts.push(`${silver}s`);
    if (copper) parts.push(`${copper}c`);

    return parts.join(' ') || '0g';
  }
}
