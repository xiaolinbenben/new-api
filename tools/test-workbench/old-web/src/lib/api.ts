import axios from 'axios';

export const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json'
  }
});

export async function getJSON<T>(url: string): Promise<T> {
  const response = await api.get<T>(url);
  return response.data;
}

export async function postJSON<T>(url: string, body: unknown): Promise<T> {
  const response = await api.post<T>(url, body);
  return response.data;
}

export async function putJSON<T>(url: string, body: unknown): Promise<T> {
  const response = await api.put<T>(url, body);
  return response.data;
}

export async function deleteJSON<T>(url: string): Promise<T> {
  const response = await api.delete<T>(url);
  return response.data;
}

