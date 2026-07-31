import { useEffect, useRef } from 'react';
import { useDialogBackClose } from './useDialogBackClose';

/**
 * The Designsystemet `<Dialog>` lifecycle in one place: opens/closes the
 * native dialog from React state, traps browser Back while open
 * (useDialogBackClose), and syncs the dialog's own close paths (X, esc,
 * backdrop) back into React. Designsystemet closes via the dialog *toggle*
 * machinery — no `close` event fires — so both events are listened to.
 * Returns the ref to attach to `<Dialog>`.
 */
export function useDsDialog(open: boolean, onClose: () => void) {
  const ref = useRef<HTMLDialogElement>(null);
  useDialogBackClose(open, onClose);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const handleClose = () => onCloseRef.current();
    const handleToggle = (e: Event) => {
      if ((e as ToggleEvent).newState === 'closed') onCloseRef.current();
    };
    el.addEventListener('close', handleClose);
    el.addEventListener('toggle', handleToggle);
    return () => {
      el.removeEventListener('close', handleClose);
      el.removeEventListener('toggle', handleToggle);
    };
  }, []);

  return ref;
}
