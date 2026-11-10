import type { Session } from '@supabase/supabase-js';
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { AppState, Platform } from 'react-native';

import { supabase } from './supabase';

type AuthState = { session: Session | null; loading: boolean };

const AuthContext = createContext<AuthState>({ session: null, loading: true });

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ session: null, loading: true });

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => setState({ session: data.session, loading: false }));
    const { data: sub } = supabase.auth.onAuthStateChange((_event, session) => setState({ session, loading: false }));

    // Refresh tokens only while the app is in the foreground (Supabase's
    // recommendation for React Native).
    const onChange = (s: string) =>
      s === 'active' ? supabase.auth.startAutoRefresh() : supabase.auth.stopAutoRefresh();
    const appState = Platform.OS === 'web' ? null : AppState.addEventListener('change', onChange);

    return () => {
      sub.subscription.unsubscribe();
      appState?.remove();
    };
  }, []);

  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>;
}

export const useAuth = () => useContext(AuthContext);
