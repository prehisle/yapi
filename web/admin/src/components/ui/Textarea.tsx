import React, { forwardRef } from 'react';

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
  hint?: string;
  required?: boolean;
  fullWidth?: boolean;
  resizable?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  (
    {
      className = '',
      label,
      error,
      hint,
      required = false,
      fullWidth = true,
      resizable = true,
      id,
      ...props
    },
    ref
  ) => {
    const textareaId = id || `textarea-${Math.random().toString(36).substr(2, 9)}`;
    const hasError = !!error;

    return (
      <div className={`field ${fullWidth ? '' : 'field--inline'} ${className}`}>
        {label && (
          <label htmlFor={textareaId} className="field__label">
            {label}
            {required && <span className="field__label--required" />}
          </label>
        )}

        <textarea
          ref={ref}
          id={textareaId}
          className={`field__textarea ${hasError ? 'field__textarea--error' : ''} ${
            !resizable ? 'resize-none' : ''
          }`}
          aria-invalid={hasError}
          aria-describedby={
            hint ? `${textareaId}-hint` : error ? `${textareaId}-error` : undefined
          }
          {...props}
        />

        {hint && !error && (
          <p id={`${textareaId}-hint`} className="field__hint">
            {hint}
          </p>
        )}

        {error && (
          <p id={`${textareaId}-error`} className="field__error" role="alert">
            {error}
          </p>
        )}
      </div>
    );
  }
);

Textarea.displayName = 'Textarea';