import { StrictMode } from 'react';
import { configureStore } from '@reduxjs/toolkit';
import { act, render, waitFor } from '@testing-library/react';
import { Provider } from 'react-redux';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import authReducer, { logout, setCredentials } from '@/modules/auth/store/authSlice';
import { AuthProvider } from './AuthProvider';

const authMocks = vi.hoisted(() => ({
  getCurrentUser: vi.fn(),
  refreshAccessToken: vi.fn(),
}));

vi.mock('@/modules/auth/services/authService', () => ({
  authService: {
    getCurrentUser: authMocks.getCurrentUser,
  },
}));

vi.mock('@/libs/http/tokenRefresh', () => ({
  refreshAccessToken: authMocks.refreshAccessToken,
}));

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function createAuthStore() {
  return configureStore({ reducer: { auth: authReducer } });
}

function renderProvider(store: ReturnType<typeof createAuthStore>) {
  return render(
    <StrictMode>
      <Provider store={store}>
        <AuthProvider>
          <div>application</div>
        </AuthProvider>
      </Provider>
    </StrictMode>
  );
}

describe('AuthProvider session restoration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    sessionStorage.clear();
    authMocks.getCurrentUser.mockResolvedValue({
      id: 'user-1',
      username: 'alice',
      email: 'alice@example.com',
      role: 'student',
    });
  });

  it('restores an initial cookie session once in StrictMode', async () => {
    authMocks.refreshAccessToken.mockResolvedValue('restored-token');
    const store = createAuthStore();

    renderProvider(store);

    await waitFor(() => expect(store.getState().auth.user?.name).toBe('alice'));
    expect(authMocks.refreshAccessToken).toHaveBeenCalledOnce();
    expect(authMocks.getCurrentUser).toHaveBeenCalledOnce();
    expect(store.getState().auth.token).toBe('restored-token');
  });

  it('does not treat an explicit logout as a session restoration request', async () => {
    const store = createAuthStore();
    store.dispatch(setCredentials({
      token: 'access-token',
      user: { id: 'user-1', name: 'Alice', role: 'student' },
    }));
    renderProvider(store);
    await waitFor(() => expect(authMocks.getCurrentUser).toHaveBeenCalledOnce());

    await act(async () => {
      store.dispatch(logout());
    });

    expect(authMocks.refreshAccessToken).not.toHaveBeenCalled();
    expect(store.getState().auth).toMatchObject({ token: null, user: null, isAuthenticated: false });
  });

  it('ignores a late initial refresh after a new login changes auth state', async () => {
    const refreshRequest = createDeferred<string | null>();
    authMocks.refreshAccessToken.mockReturnValue(refreshRequest.promise);
    const store = createAuthStore();
    renderProvider(store);
    await waitFor(() => expect(authMocks.refreshAccessToken).toHaveBeenCalledOnce());

    authMocks.getCurrentUser.mockResolvedValue({
      id: 'user-2',
      username: 'bob',
      email: 'bob@example.com',
      role: 'teacher',
    });
    await act(async () => {
      store.dispatch(setCredentials({
        token: 'new-login-token',
        user: { id: 'user-2', name: 'Bob', role: 'teacher' },
      }));
    });
    await act(async () => {
      refreshRequest.resolve('stale-restored-token');
      await refreshRequest.promise;
    });

    expect(store.getState().auth.token).toBe('new-login-token');
    expect(store.getState().auth.user?.id).toBe('user-2');
  });

  it('ignores a late initial refresh after the provider unmounts', async () => {
    const refreshRequest = createDeferred<string | null>();
    authMocks.refreshAccessToken.mockReturnValue(refreshRequest.promise);
    const store = createAuthStore();
    const view = renderProvider(store);
    await waitFor(() => expect(authMocks.refreshAccessToken).toHaveBeenCalledOnce());

    view.unmount();
    await act(async () => {
      refreshRequest.resolve('late-token');
      await refreshRequest.promise;
    });

    expect(store.getState().auth.token).toBeNull();
    expect(authMocks.getCurrentUser).not.toHaveBeenCalled();
  });
});
