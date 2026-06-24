# 管理端

React + TypeScript + Vite 管理端，用于维护后台账号、角色、菜单权限、App 用户、视频、类别、转码历史、支付套餐和订单。

## 启动

```bash
npm install
npm run dev
```

默认访问后端同源 `/api`，本地开发时需要先启动 Go 服务。

## 构建

```bash
npm run build
```

## 默认账号

```text
admin / 123456
operator / 123456
```

`admin` 拥有全部菜单和按钮权限。

## 菜单

侧边栏菜单来自后端 `admin_menus`，支持：

- `parent_id`：父子菜单
- `sort_order`：菜单排序
- `icon`：lucide-react 图标编码
- `permission`：按钮权限编码

默认主菜单包括：

```text
系统管理
  管理员
  角色
  菜单
App 管理
  用户
视频管理
  视频列表
  类别
  转码历史
支付
```

前端只会展示已有页面实现的路由；新增菜单路由前，需要先补对应页面组件和路由映射。

## 支付

支付页支持维护会员套餐和单片套餐。单片套餐可以通过可搜索下拉选择影片；订单列表会展示 App 用户名，并保留用户 ID 作为辅助信息。
