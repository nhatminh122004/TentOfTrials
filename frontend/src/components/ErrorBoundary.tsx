import React from 'react';
import { trackError } from '../services/telemetry';

export interface ErrorBoundaryFallbackProps {
  error: Error;
  errorInfo: React.ErrorInfo | null;
  reset: () => void;
  copyErrorDetails: () => Promise<void>;
  copied: boolean;
}

type ErrorBoundaryFallback =
  | React.ReactNode
  | ((props: ErrorBoundaryFallbackProps) => React.ReactNode);

interface ErrorBoundaryProps {
  children: React.ReactNode;
  fallback?: ErrorBoundaryFallback;
  onError?: (error: Error, errorInfo: React.ErrorInfo) => void;
  componentName?: string;
}

interface ErrorBoundaryState {
  error: Error | null;
  errorInfo: React.ErrorInfo | null;
  copied: boolean;
  fallbackFailed: boolean;
  resetKey: number;
}

interface FallbackRendererProps {
  fallback?: ErrorBoundaryFallback;
  fallbackProps: ErrorBoundaryFallbackProps;
}

function FallbackRenderer({ fallback, fallbackProps }: FallbackRendererProps) {
  if (typeof fallback === 'function') {
    return <>{fallback(fallbackProps)}</>;
  }

  if (fallback) {
    return <>{fallback}</>;
  }

  return <DefaultErrorFallback {...fallbackProps} />;
}

function DefaultErrorFallback({
  error,
  errorInfo,
  reset,
  copyErrorDetails,
  copied,
}: ErrorBoundaryFallbackProps) {
  return (
    <section
      role="alert"
      style={{
        border: '1px solid #7f1d1d',
        borderRadius: 8,
        background: '#1f1111',
        color: '#fee2e2',
        padding: 20,
      }}
    >
      <h2 style={{ margin: '0 0 8px', fontSize: 20 }}>Something went wrong</h2>
      <p style={{ margin: '0 0 16px', color: '#fecaca' }}>{error.message}</p>

      <details style={{ marginBottom: 16 }}>
        <summary style={{ cursor: 'pointer', color: '#fca5a5' }}>Error details</summary>
        <pre
          style={{
            marginTop: 12,
            maxHeight: 220,
            overflow: 'auto',
            whiteSpace: 'pre-wrap',
            color: '#fecaca',
            fontSize: 12,
          }}
        >
          {formatErrorDetails(error, errorInfo)}
        </pre>
      </details>

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
        <button type="button" onClick={reset}>
          Try Again
        </button>
        <button type="button" onClick={() => void copyErrorDetails()}>
          {copied ? 'Copied' : 'Copy Error Details'}
        </button>
      </div>
    </section>
  );
}

function MinimalFallback() {
  return (
    <section role="alert" style={{ padding: 20 }}>
      Something went very wrong
    </section>
  );
}

function formatErrorDetails(error: Error, errorInfo: React.ErrorInfo | null) {
  return [
    error.stack || error.message,
    errorInfo?.componentStack ? `Component stack:${errorInfo.componentStack}` : '',
  ]
    .filter(Boolean)
    .join('\n\n');
}

async function copyText(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', 'true');
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
}

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  private capturedErrors = 0;

  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = {
      error: null,
      errorInfo: null,
      copied: false,
      fallbackFailed: false,
      resetKey: 0,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { error, copied: false };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    this.capturedErrors += 1;

    if (this.capturedErrors > 1) {
      this.setState({ errorInfo, fallbackFailed: true });
      return;
    }

    this.setState({ errorInfo });

    try {
      console.error('[ErrorBoundary] Page render failed', error, errorInfo);
    } catch {
      // Console logging should never make recovery worse.
    }

    try {
      trackError(error, this.props.componentName, ['error-boundary']);
    } catch {
      // Telemetry failures should not affect the fallback UI.
    }

    try {
      this.props.onError?.(error, errorInfo);
    } catch (callbackError) {
      try {
        console.error('[ErrorBoundary] onError callback failed', callbackError);
      } catch {
        // Ignore secondary console failures.
      }
    }
  }

  private reset = () => {
    this.capturedErrors = 0;
    this.setState((state) => ({
      error: null,
      errorInfo: null,
      copied: false,
      fallbackFailed: false,
      resetKey: state.resetKey + 1,
    }));
  };

  private copyErrorDetails = async () => {
    const { error, errorInfo } = this.state;
    if (!error) return;

    try {
      await copyText(formatErrorDetails(error, errorInfo));
      this.setState({ copied: true });
    } catch (copyError) {
      try {
        console.error('[ErrorBoundary] Failed to copy error details', copyError);
      } catch {
        // Ignore secondary console failures.
      }
    }
  };

  render() {
    const { error, errorInfo, copied, fallbackFailed, resetKey } = this.state;

    if (fallbackFailed) {
      return <MinimalFallback />;
    }

    if (error) {
      return (
        <FallbackRenderer
          fallback={this.props.fallback}
          fallbackProps={{
            error,
            errorInfo,
            reset: this.reset,
            copyErrorDetails: this.copyErrorDetails,
            copied,
          }}
        />
      );
    }

    return <React.Fragment key={resetKey}>{this.props.children}</React.Fragment>;
  }
}

export default ErrorBoundary;
