import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import PromptManagementPage from '../PromptManagementPage'

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock fetch globally
global.fetch = vi.fn() as any

describe('PromptManagementPage', () => {
  beforeEach(() => {
    // Reset fetch mock
    vi.clearAllMocks()
  })

  it('should handle empty template list gracefully', async () => {
    // Mock API to return empty templates
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ templates: [] }),
    })

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText(/模板列表 \(0\)/)).toBeInTheDocument()
    })
  })

  it('should handle API error gracefully', async () => {
    // Mock API to fail
    ;(global.fetch as any).mockRejectedValueOnce(new Error('Network error'))

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
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ templates: mockTemplates }),
    })

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText('测试模板')).toBeInTheDocument()
    })

    // Mock template content API
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ content: mockContent }),
    })

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
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ templates: mockTemplates }),
    })

    render(<PromptManagementPage />)

    await waitFor(() => {
      expect(screen.getByText('测试模板')).toBeInTheDocument()
    })

    // Mock template content API to return undefined content
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ content: undefined }),
    })

    // Click on template
    const templateButton = screen.getByText('测试模板')
    fireEvent.click(templateButton)

    await waitFor(() => {
      // Should show 0 characters and 0 lines, not crash
      expect(screen.getByText(/字符数: 0/)).toBeInTheDocument()
      expect(screen.getByText(/行数: 0/)).toBeInTheDocument()
    })
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
    ;(global.fetch as any)
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ templates: mockTemplates }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ content: mockContent }),
      })

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
