import { describe, expect, test } from 'bun:test';

import { categoryColor, categoryColorMap } from './palette';

describe('categoryColorMap', () => {
  test('gives every category on screen its own color', () => {
    const ids = Array.from({ length: 16 }, (_, i) => `00000000-0000-4000-8000-${String(i).padStart(12, '0')}`);
    const colors = categoryColorMap(ids);
    expect(new Set(colors.values()).size).toBe(16);
  });

  test('keeps the hashed color when there is no clash', () => {
    const id = 'a1b2c3d4-0000-4000-8000-000000000001';
    expect(categoryColorMap([id]).get(id)).toBe(categoryColor(id));
  });

  test('does not depend on input order', () => {
    const ids = ['b-2', 'a-1', 'c-3', 'a-1'];
    expect([...categoryColorMap(ids)]).toEqual([...categoryColorMap([...ids].reverse())]);
  });
});
