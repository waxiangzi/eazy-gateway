import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      // A full page reload is required here (not just a hash change) because
      // the Pinia auth store keeps its token ref in memory. Removing the item
      // from localStorage does not update the reactive ref, so without a reload
      // the store would still report isLoggedIn === true, causing a redirect
      // loop when the router guard runs.
      window.location.assign('/')
    }
    return Promise.reject(error)
  }
)

export default client
