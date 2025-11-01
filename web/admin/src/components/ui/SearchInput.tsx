import React, { forwardRef } from 'react';

export interface SearchInputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  onClear?: () => void;
  showClearButton?: boolean;
}

export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  (
    {
      className = '',
      value,
      onClear,
      showClearButton = true,
      placeholder = '搜索...',
      ...props
    },
    ref
  ) => {
    const hasValue = value && typeof value === 'string' && value.trim().length > 0;

    const handleClear = () => {
      if (onClear) {
        onClear();
      }
      // Also trigger onChange if available to clear the input
      if (props.onChange) {
        const event = {
          target: { value: '' },
        } as React.ChangeEvent<HTMLInputElement>;
        props.onChange(event);
      }
    };

    return (
      <div className={`relative flex-1 max-w-md ${className}`}>
        <div className="absolute left-3 top-half transform-center text-gray-400">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.35-4.35" />
          </svg>
        </div>

        <input
          ref={ref}
          type="text"
          className={`field__input pl-10 ${showClearButton && hasValue ? 'pr-10' : ''}`}
          placeholder={placeholder}
          value={value}
          {...props}
        />

        {showClearButton && hasValue && (
          <button
            type="button"
            className="absolute right-3 top-half -translate-y-half text-gray-400 hover:text-gray-600 transition-colors"
            onClick={handleClear}
            aria-label="清除搜索"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        )}
      </div>
    );
  }
);

SearchInput.displayName = 'SearchInput';