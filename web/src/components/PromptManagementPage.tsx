import { useEffect, useState } from 'react'
import { toast } from 'sonner'

interface PromptTemplate {
  name: string
  content: string
  display_name?: { [key: string]: string }
  description?: { [key: string]: string }
}

export default function PromptManagementPage() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [selectedTemplate, setSelectedTemplate] = useState<PromptTemplate | null>(null)
  const [editContent, setEditContent] = useState('')
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [newTemplateName, setNewTemplateName] = useState('')
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)

  // 加载模板列表
  const loadTemplates = async () => {
    try {
      const response = await fetch('/api/prompt-templates')
      const data = await response.json()
      setTemplates(data.templates || [])
    } catch (error) {
      console.error('加载模板失败:', error)
      toast.error('加载模板失败')
    }
  }

  useEffect(() => {
    loadTemplates()
  }, [])

  // 选择模板
  const handleSelectTemplate = (template: PromptTemplate) => {
    setSelectedTemplate(template)
    setEditContent(template.content)
  }

  // 保存模板
  const handleSave = async () => {
    if (!selectedTemplate) return

    try {
      const response = await fetch(`/api/prompt-templates/${selectedTemplate.name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: editContent }),
      })

      if (response.ok) {
        toast.success('保存成功')
        loadTemplates()
      } else {
        const error = await response.json()
        toast.error(error.error || '保存失败')
      }
    } catch (error) {
      console.error('保存失败:', error)
      toast.error('保存失败')
    }
  }

  // 创建新模板
  const handleCreate = async () => {
    if (!newTemplateName.trim()) {
      toast.error('请输入模板名称')
      return
    }

    try {
      const response = await fetch('/api/prompt-templates', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newTemplateName,
          content: editContent || '# 新模板\n\n请输入您的提示词内容...',
        }),
      })

      if (response.ok) {
        toast.success('创建成功')
        setIsCreateModalOpen(false)
        setNewTemplateName('')
        loadTemplates()
      } else {
        const error = await response.json()
        toast.error(error.error || '创建失败')
      }
    } catch (error) {
      console.error('创建失败:', error)
      toast.error('创建失败')
    }
  }

  // 删除模板
  const handleDelete = async () => {
    if (!selectedTemplate) return

    try {
      const response = await fetch(`/api/prompt-templates/${selectedTemplate.name}`, {
        method: 'DELETE',
      })

      if (response.ok) {
        toast.success('删除成功')
        setIsDeleteModalOpen(false)
        setSelectedTemplate(null)
        setEditContent('')
        loadTemplates()
      } else {
        const error = await response.json()
        toast.error(error.error || '删除失败')
      }
    } catch (error) {
      console.error('删除失败:', error)
      toast.error('删除失败')
    }
  }

  return (
    <div className="min-h-screen p-6" style={{ background: '#0B0E11', color: '#EAECEF' }}>
      {/* Header */}
      <div className="max-w-7xl mx-auto mb-8">
        <h1 className="text-3xl font-bold mb-2">💬 提示词管理</h1>
        <p className="text-gray-400">管理您的 AI 交易策略提示词模板</p>
      </div>

      {/* Actions */}
      <div className="max-w-7xl mx-auto mb-6 flex gap-4">
        <button
          onClick={() => setIsCreateModalOpen(true)}
          className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
          style={{ background: '#F0B90B', color: '#000' }}
        >
          + 新建模板
        </button>
        <button
          onClick={loadTemplates}
          className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
          style={{ background: 'rgba(240, 185, 11, 0.1)', color: '#F0B90B', border: '1px solid #F0B90B' }}
        >
          🔄 刷新
        </button>
      </div>

      {/* Main Content: Template List + Editor */}
      <div className="max-w-7xl mx-auto grid grid-cols-12 gap-6">
        {/* Template List (Left Sidebar) */}
        <div className="col-span-3 bg-[#1E2329] border border-[#2B3139] rounded-lg p-4">
          <h2 className="text-lg font-bold mb-4">📁 模板列表 ({templates.length})</h2>
          <div className="space-y-2">
            {templates.map((template) => (
              <button
                key={template.name}
                onClick={() => handleSelectTemplate(template)}
                className={`w-full text-left px-3 py-2 rounded transition-all ${
                  selectedTemplate?.name === template.name
                    ? 'bg-yellow-500 bg-opacity-20 border border-yellow-500'
                    : 'hover:bg-gray-700'
                }`}
                style={{
                  color: selectedTemplate?.name === template.name ? '#F0B90B' : '#EAECEF',
                }}
              >
                {template.name === 'default' && '⭐ '}
                {template.display_name?.zh || template.name}
              </button>
            ))}
          </div>
        </div>

        {/* Editor (Right Panel) */}
        <div className="col-span-9 bg-[#1E2329] border border-[#2B3139] rounded-lg p-6">
          {selectedTemplate ? (
            <>
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-xl font-bold">
                  📝 {selectedTemplate.display_name?.zh || selectedTemplate.name}
                </h2>
                <div className="flex gap-3">
                  <button
                    onClick={handleSave}
                    className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
                    style={{ background: '#0ECB81', color: '#FFF' }}
                  >
                    💾 保存
                  </button>
                  {selectedTemplate.name !== 'default' && (
                    <button
                      onClick={() => setIsDeleteModalOpen(true)}
                      className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
                      style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D', border: '1px solid #F6465D' }}
                    >
                      🗑️ 删除
                    </button>
                  )}
                </div>
              </div>

              {selectedTemplate.description?.zh && (
                <p className="text-sm text-gray-400 mb-4">{selectedTemplate.description.zh}</p>
              )}

              <textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                className="w-full h-[500px] p-4 rounded font-mono text-sm"
                style={{
                  background: '#0B0E11',
                  color: '#EAECEF',
                  border: '1px solid #2B3139',
                  resize: 'none',
                }}
              />

              <div className="mt-2 flex justify-between text-xs text-gray-500">
                <span>字符数: {editContent.length}</span>
                <span>行数: {editContent.split('\n').length}</span>
              </div>
            </>
          ) : (
            <div className="flex flex-col items-center justify-center h-[500px] text-gray-500">
              <p className="text-lg">请从左侧选择一个模板</p>
            </div>
          )}
        </div>
      </div>

      {/* Create Modal */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-[#1E2329] border border-[#2B3139] rounded-lg p-6 w-96">
            <h2 className="text-xl font-bold mb-4">新建模板</h2>
            <input
              type="text"
              value={newTemplateName}
              onChange={(e) => setNewTemplateName(e.target.value)}
              placeholder="输入模板名称（英文）"
              className="w-full px-3 py-2 rounded mb-4"
              style={{ background: '#0B0E11', color: '#EAECEF', border: '1px solid #2B3139' }}
            />
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setIsCreateModalOpen(false)}
                className="px-4 py-2 rounded"
                style={{ background: 'rgba(255,255,255,0.1)', color: '#EAECEF' }}
              >
                取消
              </button>
              <button
                onClick={handleCreate}
                className="px-4 py-2 rounded font-semibold"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                创建
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {isDeleteModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-[#1E2329] border border-[#2B3139] rounded-lg p-6 w-96">
            <h2 className="text-xl font-bold mb-4">确认删除</h2>
            <p className="mb-4 text-gray-400">
              确定要删除模板「{selectedTemplate?.name}」吗？此操作无法撤销。
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setIsDeleteModalOpen(false)}
                className="px-4 py-2 rounded"
                style={{ background: 'rgba(255,255,255,0.1)', color: '#EAECEF' }}
              >
                取消
              </button>
              <button
                onClick={handleDelete}
                className="px-4 py-2 rounded font-semibold"
                style={{ background: '#F6465D', color: '#FFF' }}
              >
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
