type ClassValue =
  | string
  | number
  | boolean
  | undefined
  | null
  | ClassValue[]
  | { [key: string]: boolean | undefined | null };

function toClassArray(value: ClassValue): string[] {
  if (value == null) return [];

  if (typeof value === 'string') return [value];
  if (typeof value === 'number') return [value.toString()];
  if (typeof value === 'boolean') return [];

  if (Array.isArray(value)) {
    return value.flatMap(toClassArray);
  }

  if (typeof value === 'object') {
    return Object.entries(value)
      .filter(([, condition]) => Boolean(condition))
      .map(([key]) => key);
  }

  return [];
}

export function cn(...inputs: ClassValue[]): string {
  return inputs.flatMap(toClassArray).filter(Boolean).join(' ');
}