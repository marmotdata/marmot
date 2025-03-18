import { auth } from './stores/auth';
import { goto } from '$app/navigation';

export async function fetchApi(endpoint: string, options: RequestInit = {}) {
  const token = auth.getToken();

  const headers = {
    'Content-Type': 'application/json',
    ...options.headers
  };

  // Only add Authorization header if we have a token
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`/api/v1${endpoint}`, {
    ...options,
    headers
  });

  if (response.status === 401) {
    auth.clearToken();
    goto('/login');
    throw new Error('Unauthorized');
  }

  return response;
}
