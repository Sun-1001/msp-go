import { configureStore } from '@reduxjs/toolkit';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Provider } from 'react-redux';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import authReducer, { setCredentials } from '@/modules/auth/store/authSlice';
import { useAuth } from './useAuth';

const authServiceMock = vi.hoisted(() => ({
  logout: vi.fn(),
}));

vi.mock('@/modules/auth/services/authService', () => ({
  authService: authServiceMock,
}));

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function AuthHarness({ logoutRedirect = '/welcome' }: { logoutRedirect?: string }) {
  const location = useLocation();
  const { handleLogout, isLoggingOut } = useAuth(logoutRedirect);

  return (
    <>
      <span data-testid="location">{location.pathname}</span>
      <button type="button" disabled={isLoggingOut} onClick={() => void handleLogout()}>
        {isLoggingOut ? '退出中...' : '退出登录'}
      </button>
    </>
  );
}

function createAuthenticatedStore() {
  const store = configureStore({ reducer: { auth: authReducer } });
  store.dispatch(setCredentials({
    token: 'access-token',
    user: { id: 'user-1', name: 'Alice', role: 'student' },
  }));
  return store;
}

describe('useAuth logout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    sessionStorage.clear();
  });

  it('waits for server logout, clears local auth, and replaces the route', async () => {
    const logoutRequest = createDeferred<void>();
    authServiceMock.logout.mockReturnValue(logoutRequest.promise);
    const store = createAuthenticatedStore();

    render(
      <Provider store={store}>
        <MemoryRouter initialEntries={['/my-class']}>
          <AuthHarness />
        </MemoryRouter>
      </Provider>
    );

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));

    expect(authServiceMock.logout).toHaveBeenCalledOnce();
    expect(screen.getByRole('button', { name: '退出中...' })).toBeDisabled();
    expect(store.getState().auth.isAuthenticated).toBe(true);

    await act(async () => {
      logoutRequest.resolve();
      await logoutRequest.promise;
    });

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/welcome'));
    expect(store.getState().auth).toMatchObject({ token: null, user: null, isAuthenticated: false });
    expect(sessionStorage.getItem('auth_token')).toBeNull();
  });

  it('always clears local auth when the server request rejects', async () => {
    authServiceMock.logout.mockRejectedValue(new Error('server unavailable'));
    const store = createAuthenticatedStore();

    render(
      <Provider store={store}>
        <MemoryRouter initialEntries={['/admin/dashboard']}>
          <AuthHarness logoutRedirect="/admin" />
        </MemoryRouter>
      </Provider>
    );

    fireEvent.click(screen.getByRole('button', { name: '退出登录' }));

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/admin'));
    expect(store.getState().auth).toMatchObject({ token: null, user: null, isAuthenticated: false });
  });
});
