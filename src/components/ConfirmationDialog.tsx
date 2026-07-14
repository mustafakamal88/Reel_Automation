import { useEffect, useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

type ConfirmationIntent = 'standard' | 'destructive';

interface ConfirmationDialogProps {
  open: boolean;
  title: string;
  description: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  intent?: ConfirmationIntent;
  pending?: boolean;
  pendingLabel?: string;
  errorMessage?: string | null;
  onConfirm: () => void;
  onCancel: () => void;
}

const focusableSelector = [
  'button:not([disabled])',
  '[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

export function ConfirmationDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Cancel',
  intent = 'standard',
  pending = false,
  pendingLabel = 'Working...',
  errorMessage,
  onConfirm,
  onCancel,
}: ConfirmationDialogProps) {
  const titleID = useId();
  const descriptionID = useId();
  const dialogRef = useRef<HTMLDivElement | null>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);
  const onCancelRef = useRef(onCancel);
  const pendingRef = useRef(pending);

  useEffect(() => {
    onCancelRef.current = onCancel;
  }, [onCancel]);

  useEffect(() => {
    pendingRef.current = pending;
  }, [pending]);

  useEffect(() => {
    if (!open) return;

    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const dialog = dialogRef.current;
    const focusable = dialog ? Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector)) : [];
    const initialFocus = focusable.find(element => element.dataset.autofocus === 'true') || focusable[0] || dialog;
    initialFocus?.focus();

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        if (!pendingRef.current) {
          event.preventDefault();
          onCancelRef.current();
        }
        return;
      }

      if (event.key !== 'Tab' || !dialogRef.current) return;
      const items = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(focusableSelector));
      if (items.length === 0) {
        event.preventDefault();
        dialogRef.current.focus();
        return;
      }

      const first = items[0];
      const last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      previousFocusRef.current?.focus();
    };
  }, [open]);

  if (!open) return null;

  return createPortal(
    <div className="confirmation-layer">
      <button
        className="confirmation-backdrop"
        type="button"
        aria-label={`Cancel ${title}`}
        disabled={pending}
        onClick={onCancel}
      />
      <div
        ref={dialogRef}
        className={`confirmation-dialog ${intent === 'destructive' ? 'is-destructive' : 'is-standard'}`}
        role={intent === 'destructive' ? 'alertdialog' : 'dialog'}
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={descriptionID}
        tabIndex={-1}
      >
        <div className="confirmation-dialog-header">
          <span className="confirmation-mark" aria-hidden="true" />
          <div>
            <h2 id={titleID}>{title}</h2>
            <div id={descriptionID} className="confirmation-description">{description}</div>
          </div>
        </div>
        {errorMessage && <div className="confirmation-error" role="alert">{errorMessage}</div>}
        {pending && <div className="confirmation-pending" role="status" aria-live="polite">{pendingLabel}</div>}
        <div className="confirmation-actions">
          <button className="generate-btn secondary" type="button" onClick={onCancel} disabled={pending} data-autofocus="true">
            {cancelLabel}
          </button>
          <button
            className={`generate-btn ${intent === 'destructive' ? 'danger' : 'idle'}`}
            type="button"
            onClick={onConfirm}
            disabled={pending}
          >
            {pending ? pendingLabel : confirmLabel}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
