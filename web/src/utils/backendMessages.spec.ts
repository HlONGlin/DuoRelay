import { afterEach, describe, expect, it } from 'vitest'
import { translateBackendMessage } from './backendMessages'

afterEach(() => localStorage.removeItem('duorelay-language'))

describe('backend messages', () => {
  it('keeps Chinese and unknown diagnostic details intact', () => {
    expect(translateBackendMessage('请勿重复选择同一个节点')).toBe('请勿重复选择同一个节点')
    localStorage.setItem('duorelay-language', 'en')
    expect(translateBackendMessage('unexpected diagnostic')).toBe('unexpected diagnostic')
  })

  it('translates chain diagnostics without changing node names', () => {
    localStorage.setItem('duorelay-language', 'en')
    expect(translateBackendMessage('第 2 跳「A」需要 UDP 传输，但前一跳「B」无法转发 UDP；请更换入口或调整顺序'))
      .toBe('Hop 2 "A" requires UDP, but previous hop "B" cannot forward UDP; change the entry or reorder the chain')
  })

  it('translates configuration and authentication errors from the API', () => {
    localStorage.setItem('duorelay-language', 'en')
    expect(translateBackendMessage('invalid input: 精简版不支持 vless')).toBe('invalid input: Lite does not support vless')
    expect(translateBackendMessage('请先创建管理员账号')).toBe('Create an administrator account first')
    expect(translateBackendMessage('第 3 跳未能生成配置')).toBe('Hop 3 could not generate configuration')
  })
})
