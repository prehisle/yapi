import React from 'react';
import { cn } from '../../utils/cn';

export interface FormSectionProps {
  title: string;
  description?: string;
  children: React.ReactNode;
  className?: string;
  collapsible?: boolean;
  defaultCollapsed?: boolean;
}

export const FormSection: React.FC<FormSectionProps> = ({
  title,
  description,
  children,
  className = '',
  collapsible = false,
  defaultCollapsed = false,
}) => {
  const [isCollapsed, setIsCollapsed] = React.useState(defaultCollapsed);

  return (
    <div className={cn('form-section', className)}>
      <div className="flex items-center justify-between">
        <div>
          <h3 className="form-section__title">{title}</h3>
          {description && (
            <p className="form-section__description">{description}</p>
          )}
        </div>
        {collapsible && (
          <button
            type="button"
            className="text-gray-400 hover:text-gray-600 transition-colors"
            onClick={() => setIsCollapsed(!isCollapsed)}
            aria-label={isCollapsed ? '展开' : '折叠'}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className={cn('transition-transform', isCollapsed ? '-rotate-90' : '')}
            >
              <path d="M6 9l6 6 6-6" />
            </svg>
          </button>
        )}
      </div>

      {!isCollapsed && (
        <div className="mt-4">
          {children}
        </div>
      )}
    </div>
  );
};