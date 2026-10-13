import { releaseUrl, snippetFrom } from './format'

describe('releaseUrl', () => {
  it("links to the tag's GitHub Releases page", () => {
    expect(releaseUrl('owner/repo', 'v1.2.3')).toBe('https://github.com/owner/repo/releases/tag/v1.2.3')
  })
})

describe('snippetFrom', () => {
  it('collapses whitespace onto one line', () => {
    expect(snippetFrom('  one\n\n  two  ')).toBe('one two')
  })

  it('cuts a long body to the limit, ending in an ellipsis', () => {
    const snippet = snippetFrom('a'.repeat(200), 10)
    expect(snippet).toHaveLength(10)
    expect(snippet.endsWith('…')).toBe(true)
  })
})
