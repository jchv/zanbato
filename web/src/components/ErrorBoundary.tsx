import { Component, type ErrorInfo, type ReactNode } from "react";

import { createLogger } from "../log";
import "./errorBoundary.css";

const log = createLogger("error-boundary");

export interface ErrorBoundaryProps {
  fallbackTitle: string | undefined;
  children: ReactNode;
}

interface State {
  error: Error | null;
  componentStack: string | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, State> {
  override state: State = { error: null, componentStack: null };

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    log.error("caught:", error, info.componentStack);
    this.setState({ componentStack: info.componentStack ?? null });
  }

  reset = (): void => {
    this.setState({ error: null, componentStack: null });
  };

  override render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="error-boundary">
        <div className="error-boundary-title">
          {this.props.fallbackTitle ?? "Component crashed"}
        </div>
        <pre className="error-boundary-message">{this.state.error.message}</pre>
        {this.state.componentStack && (
          <details className="error-boundary-details">
            <summary>Component stack</summary>
            <pre className="error-boundary-stack">
              {this.state.componentStack}
            </pre>
          </details>
        )}
        <button
          type="button"
          className="error-boundary-retry"
          onClick={this.reset}
        >
          Retry
        </button>
      </div>
    );
  }
}

export function withErrorBoundary<P extends object>(
  Inner: React.FC<P>,
  title?: string,
): React.FC<P> {
  const Wrapped: React.FC<P> = (props) => (
    <ErrorBoundary fallbackTitle={title}>
      <Inner {...props} />
    </ErrorBoundary>
  );
  Wrapped.displayName = `withErrorBoundary(${Inner.displayName ?? Inner.name})`;
  return Wrapped;
}
