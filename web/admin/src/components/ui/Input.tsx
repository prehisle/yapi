import React, { forwardRef } from 'react';

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  hint?: string;
  required?: boolean;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
  fullWidth?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  (
    {
      className = '',
      label,
      error,
      hint,
      required = false,
      leftIcon,
      rightIcon,
      fullWidth = true,
      id,
      ...props
    },
    ref
  ) => {
    const inputId = id || `input-${Math.random().toString(36).substr(2, 9)}`;
    const hasError = !!error;

    return (
      <div className={`field ${fullWidth ? '' : 'field--inline'} ${className}`}>
        {label && (
          <label htmlFor={inputId} className="field__label">
            {label}
            {required && <span className="field__label--required" />}
          </label>
        )}

        <div className="relative">
          {leftIcon && (
            <div className="absolute left-3 top-half transform-center text-gray-400">
              {leftIcon}
            </div>
          )}

          <input
            ref={ref}
            id={inputId}
            className={`field__input ${hasError ? 'field__input--error' : ''} ${
              leftIcon ? 'pl-10' : ''
            } ${rightIcon ? 'pr-10' : ''}`}
            aria-invalid={hasError}
            aria-describedby={
              hint ? `${inputId}-hint` : error ? `${inputId}-error` : undefined
            }
            {...props}
          />

          {rightIcon && (
            <div className="absolute right-3 top-half -translate-y-half text-gray-400">
              {rightIcon}
            </div>
          )}
        </div>

        {hint && !error && (
          <p id={`${inputId}-hint`} className="field__hint">
            {hint}
          </p>
        )}

        {error && (
          <p id={`${inputId}-error`} className="field__error" role="alert">
            {error}
          </p>
        )}
      </div>
    );
  }
);

Input.displayName = 'Input';