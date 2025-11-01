import React, { forwardRef } from 'react';

export interface SelectProps
  extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  error?: string;
  hint?: string;
  required?: boolean;
  fullWidth?: boolean;
  options: Array<{
    value: string;
    label: string;
    disabled?: boolean;
  }>;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  (
    {
      className = '',
      label,
      error,
      hint,
      required = false,
      fullWidth = true,
      options,
      id,
      ...props
    },
    ref
  ) => {
    const selectId = id || `select-${Math.random().toString(36).substr(2, 9)}`;
    const hasError = !!error;

    return (
      <div className={`field ${fullWidth ? '' : 'field--inline'} ${className}`}>
        {label && (
          <label htmlFor={selectId} className="field__label">
            {label}
            {required && <span className="field__label--required" />}
          </label>
        )}

        <select
          ref={ref}
          id={selectId}
          className={`field__select ${hasError ? 'field__select--error' : ''}`}
          aria-invalid={hasError}
          aria-describedby={
            hint ? `${selectId}-hint` : error ? `${selectId}-error` : undefined
          }
          {...props}
        >
          {options.map((option) => (
            <option
              key={option.value}
              value={option.value}
              disabled={option.disabled}
            >
              {option.label}
            </option>
          ))}
        </select>

        {hint && !error && (
          <p id={`${selectId}-hint`} className="field__hint">
            {hint}
          </p>
        )}

        {error && (
          <p id={`${selectId}-error`} className="field__error" role="alert">
            {error}
          </p>
        )}
      </div>
    );
  }
);

Select.displayName = 'Select';