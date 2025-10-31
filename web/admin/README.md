# YAPI 管理后台前端

基于 React + TypeScript + Vite 的单页应用，为 `/admin` API 提供登录与规则管理界面，集成 JWT 登录、规则 CRUD、通知与确认交互。

## 快速开始

```bash
npm install        # 安装依赖
npm run dev        # 本地开发，默认监听 http://localhost:5173
npm run lint       # 代码检查（ESLint）
npm run build      # 生成产物（dist/）
```

> Vite dev server 默认将 `/admin` 请求代理到 `http://localhost:8080`，可通过设置 `VITE_API_BASE_URL` 修改。

## 目录结构

- `src/pages/`：页面组件（`LoginPage`, `RulesPage` 等）。
- `src/context/`：全局状态（如 `AuthProvider` 保存 access token）。
- `src/lib/`：通用工具，例如 `apiClient` 封装 fetch 与错误处理。
- `src/types/`：与后端交互的数据类型声明。

## 功能摘要

- 登录：输入管理员账户密码，调用 `/admin/login` 获取短期 Bearer Token。
- 规则列表：支持查询、分页、刷新，展示规则状态并提供启用/禁用、编辑、删除操作。
- 规则表单：可配置路径、方法、目标地址以及头部修改、JSON 动作；提交前会校验格式并给出提示。
- 通知中心：操作成功或失败时在右上角弹出 Toast，删除需二次确认。

## 配置提示

- 头部配置字段使用 `key=value` 逐行输入；移除头部与 JSON 字段支持逗号或换行分隔。
- `override_json` 字段需填写合法 JSON 对象（例如 `{ "model": "gpt-4.1" }`）。
- 可通过 `.env` 设置 `VITE_API_BASE_URL` 指向后端服务地址。

## 后续规划

- 丰富规则详情展示、导入导出能力。
- 角色权限与操作审计。
- 国际化与主题定制。
