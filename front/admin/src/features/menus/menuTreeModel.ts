import type { Menu } from '../../adminTypes'

export type MenuNodeType = Menu & { children: MenuNodeType[] }

export function buildMenuTree(menus: Menu[]): MenuNodeType[] {
  const map = new Map<number, MenuNodeType>()
  menus.forEach((menu) => map.set(menu.id, { ...menu, children: [] }))
  const roots: MenuNodeType[] = []

  map.forEach((node) => {
    const parent = map.get(node.parentId)
    if (parent) {
      parent.children.push(node)
      return
    }
    roots.push(node)
  })

  sortMenuNodes(roots)
  return roots
}

function sortMenuNodes(nodes: MenuNodeType[]) {
  nodes.sort((left, right) => left.sortOrder - right.sortOrder || left.id - right.id)
  nodes.forEach((node) => sortMenuNodes(node.children))
}
