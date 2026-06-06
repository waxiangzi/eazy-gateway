import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
})

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      const url = error.config?.url || ''
      // Don't redirect for the auth-check endpoint; checkAuth() handles that.
      const pathname = url.split('?')[0]
      if (pathname !== '/me') {
        // Use hash navigation instead of full-page reload to avoid flashing loops.
        window.location.hash = '#/login'
      }
    }
    return Promise.reject(error)
  }
)

export default client
