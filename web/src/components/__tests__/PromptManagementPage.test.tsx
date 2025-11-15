import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import PromptManagementPage from '../PromptManagementPage'
import { httpClient } from '../../lib/httpClient'

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock httpClient
vi.mock('../../lib/httpClient', () => ({
  httpClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

// Mock localStorage
const mockLocalStorage = {
  getItem: vi.fn(() => 'mock-token'),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}
Object.defineProperty(window, 'localStorage', {
  value: mockLocalStorage,
})

describe('PromptManagementPage', () => {
  beforeEach(() => {
    // Reset all mocks
    vi.clearAllMocks()
  })

  it('should handle empty template list gracefully', async () => {
    // Mock API to return empty templates
    vi.mocked(httpClient.get).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ templates: [] }),
    } as Response)

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText(/模板列表 \(0\)/)).toBeInTheDocument()
    })
  })

  it('should handle API error gracefully', async () => {
    // Mock API to fail
    vi.mocked(httpClient.get).mockRejectedValueOnce(new Error('Network error'))

    render(<PromptManagementPage />)

    await waitFor(() => {
      // Should still render the page without crashing
      expect(screen.getByText(/模板列表/)).toBeInTheDocument()
    })
  })

  it('should load template content when selected', async () => {
    const mockTemplates = [
      {
        name: 'test-template',
        display_name: { zh: '测试模板', en: 'Test Template' },
        description: { zh: '测试用', en: 'For testing' },
      },
    ]

    const mockContent = '# 测试模板\n\n这是测试内容'

    // Mock template list API
    vi.mocked(httpClient.get).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ templates: mockTemplates }),
    } as Response)

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText('测试模板')).toBeInTheDocument()
    })

    // Mock template content API
    vi.mocked(httpClient.get).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ content: mockContent }),
    } as Response)

    // Click on template
    const templateButton = screen.getByText('测试模板')
    fireEvent.click(templateButton)

    await waitFor(() => {
      const textarea = screen.getByRole('textbox') as HTMLTextAreaElement
      expect(textarea.value).toBe(mockContent)
    })
  })

  it('should handle undefined editContent gracefully', async () => {
    const mockTemplates = [
      {
        name: 'test-template',
        display_name: { zh: '测试模板' },
      },
    ]

    // Mock template list API
    vi.mocked(httpClient.get).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ templates: mockTemplates }),
    } as Response)

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText('测试模板')).toBeInTheDocument()
    })

    // Mock template content API to return undefined content
    vi.mocked(httpClient.get).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ content: undefined }),
    } as Response)

    // Click on template
    const templateButton = screen.getByText('测试模板')
    fireEvent.click(templateButton)

    await waitFor(() => {
      // Should show 0 characters and 0 lines, not crash
      expect(screen.getByText(/字符数:/)).toBeInTheDocument()
    })

    // Verify stats display correctly (using regex to be more flexible)
    const statsText = screen.getByText(/字符数:/).textContent
    expect(statsText).toContain('字符数: 0')
  })

  it('should display character and line count correctly', async () => {
    const mockTemplates = [
      {
        name: 'test',
        display_name: { zh: '测试' },
      },
    ]

    const mockContent = 'Line 1\nLine 2\nLine 3'

    // Mock APIs
    vi.mocked(httpClient.get)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ templates: mockTemplates }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ content: mockContent }),
      } as Response)

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText('测试')).toBeInTheDocument()
    })

    const templateButton = screen.getByText('测试')
    fireEvent.click(templateButton)

    await waitFor(() => {
      expect(screen.getByText(/字符数: 20/)).toBeInTheDocument()
      expect(screen.getByText(/行数: 3/)).toBeInTheDocument()
    })
  })
})
