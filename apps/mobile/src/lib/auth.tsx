import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';

import { getSession, onSessionChange, type Session } from './session';

type AuthState = { session: Session | null; loading: boolean };

const AuthContext = createContext<AuthState>({ session: null, loading: true });

// Tokens are refreshed on demand (when the API needs one), so there is no
// background timer here.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ session: null, loading: true });

  useEffect(() => {
    getSession().then((session) => setState({ session, loading: false }));
    return onSessionChange((session) => setState({ session, loading: false }));
  }, []);

  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>;
}

export const useAuth = () => useContext(AuthContext);
