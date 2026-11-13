import { createApi } from './api';
import { config } from './config';
import { accessToken } from './session';

export const api = createApi({ baseUrl: config.apiUrl, getToken: accessToken });
