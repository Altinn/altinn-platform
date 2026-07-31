import { useEffect, useRef } from 'react';

/**
 * Makes the browser Back button close an open dialog instead of leaving the
 * page: opening pushes a same-URL history entry, Back pops it (we close the
 * dialog), and a UI-side close (X / esc / backdrop) consumes the entry so
 * history stays clean.
 */
export function useDialogBackClose(open: boolean, onClose: () => void) {
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useEffect(() => {
    if (!open) return;
    // Idempotent: StrictMode double-invocation (and any re-entry) must not
    // stack a second sentinel, or one close leaves a stale entry behind.
    if (!(window.history.state as { dialog?: boolean } | null)?.dialog) {
      window.history.pushState({ dialog: true }, '');
    }
    const onPop = () => closeRef.current();
    window.addEventListener('popstate', onPop);
    return () => {
      window.removeEventListener('popstate', onPop);
      // Closed via the UI: the sentinel entry is still current — consume it.
      // Closed via Back: it is already gone, so going back again would leave
      // the page the user is looking at.
      if ((window.history.state as { dialog?: boolean } | null)?.dialog) {
        window.history.back();
      }
    };
  }, [open]);
}
