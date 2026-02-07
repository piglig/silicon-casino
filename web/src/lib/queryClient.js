import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      gcTime: 5 * 60 * 1000,
      retry: (count, err) => {
        const status = err?.status || 0
        if (status >= 400 && status < 500) return false
        return count < 1
      },
      refetchOnWindowFocus: false
    }
  }
})
