# ISDP 主题设计指南

## 🎨 主题定位

**C 清新活力风配色 + A 现代科技感细节**

融合清新自然的视觉感受与科技感的精致细节，打造专业而不失活力的开发者工具体验。

---

## 📋 配色方案

### 主色调
```
Primary:     #10b981 (翡翠绿)
Primary Dark: #059669 (深绿)
Primary Light:#a7f3d0 (浅绿)
```

### 辅助色
```
Success:     #10b981 (翡翠绿)
Info:        #14b8a6 (青绿色)
Warning:     #f59e0b (琥珀黄)
Error:       #ef4444 (珊瑚红)
```

### 中性色
```
Background:  #f0fdf4 → #ecfdf5 → #e0f2f1 (渐变)
Surface:     #ffffff
Border:      #d1fae5 / #a7f3d0
Text:        #047857 (主) / #6b7280 (次)
```

---

## 🎯 设计特征

### 1. 圆角系统
```
小：   6px   - 标签、小按钮
中：   12px  - 输入框、普通按钮
大：   16px  - 卡片、表格
超大：20px  - 模态框、抽屉
特大：24px  - 特殊容器
```

### 2. 阴影层次
```
柔和：   0 4px 16px rgba(16, 185, 129, 0.08)
中等：   0 8px 24px rgba(16, 185, 129, 0.12)
强烈：   0 12px 40px rgba(16, 185, 129, 0.18)
发光：   0 0 20px rgba(16, 185, 129, 0.4) + 外扩光晕
```

### 3. 渐变效果
```
主渐变：   linear-gradient(135deg, #10b981, #059669)
青绿渐变： linear-gradient(135deg, #10b981, #14b8a6)
科技渐变： linear-gradient(135deg, #6366f1, #10b981)
```

### 4. 科技感细节
- **发光效果**: 聚焦/悬停时的柔和光晕
- **悬停浮动**: 卡片悬停时轻微上浮
- **渐变边框**: 微妙的彩色边框过渡
- **动态阴影**: 随交互变化的多层阴影

---

## 🧩 组件样式规范

### 按钮 (Button)
```css
- 圆角：12px
- 字重：600
- 背景：linear-gradient(135deg, #10b981, #059669)
- 阴影：0 4px 14px rgba(16, 185, 129, 0.35)
- 悬停：上浮 3px + 阴影增强
```

### 卡片 (Card)
```css
- 圆角：16px
- 阴影：0 8px 24px rgba(16, 185, 129, 0.12)
- 悬停：上浮 6px + 外层光晕
- 头部：淡绿渐变背景
```

### 输入框 (Input)
```css
- 圆角：12px
- 边框：#d1fae5
- 聚焦：10b981 边框 + 3px 外发光
- 悬停：轻微发光效果
```

### 表格 (Table)
```css
- 圆角：16px
- 表头：linear-gradient(180deg, #f0fdf4, #ecfdf5)
- 行悬停：rgba(16, 185, 129, 0.08) + 内发光
- 整体：外阴影保护
```

### 菜单 (Menu)
```css
- 圆角：12px
- 悬停：10% 透明度渐变
- 选中：20% 透明度 + 右侧光晕指示条
- 过渡：0.3s cubic-bezier
```

### 进度条 (Progress)
```css
- 圆角：6px
- 背景：linear-gradient(90deg, #10b981, #14b8a6, #0d9488)
- 阴影：0 0 10px rgba(16, 185, 129, 0.4)
- 末端：白色高光效果
```

### 标签 (Tag)
```css
- 圆角：8px
- 默认：#ecfdf5 背景 + #047857 文字
- 边框：#a7f3d0
- 字重：600
```

---

## ✨ 动画效果

### fadeIn - 页面加载
```css
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(20px); }
  to   { opacity: 1; transform: translateY(0); }
}
```

### glow - 科技发光
```css
@keyframes glow {
  0%, 100% { box-shadow: 0 0 10px rgba(16,185,129,0.3); }
  50%      { box-shadow: 0 0 20px rgba(16,185,129,0.5), 0 0 40px rgba(16,185,129,0.2); }
}
```

### float - 悬浮效果
```css
@keyframes float {
  0%, 100% { transform: translateY(0); }
  50%      { transform: translateY(-8px); }
}
```

### shimmer - 闪烁加载
```css
@keyframes shimmer {
  0%   { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
```

---

## 🛠️ 工具类

### 圆角
- `.rounded-sm` - 6px
- `.rounded-md` - 12px
- `.rounded-lg` - 16px
- `.rounded-xl` - 20px
- `.rounded-2xl` - 24px

### 阴影
- `.shadow-soft` - 柔和阴影
- `.shadow-medium` - 中等阴影
- `.shadow-strong` - 强烈阴影
- `.shadow-glow-green` - 绿色发光
- `.shadow-glow-teal` - 青色发光
- `.shadow-glow-purple` - 紫色发光

### 渐变背景
- `.gradient-primary` - 主色渐变
- `.gradient-success` - 成功渐变
- `.gradient-warning` - 警告渐变
- `.gradient-danger` - 危险渐变
- `.gradient-info` - 信息渐变
- `.gradient-tech` - 科技感紫绿渐变

### 特效
- `.glass-effect` - 玻璃态效果
- `.glass-dark` - 深色玻璃态
- `.card-hover-glow` - 悬停发光卡片
- `.border-gradient` - 渐变边框
- `.halo-decoration` - 光晕装饰
- `.selected-glow` - 选中发光
- `.glow-effect` - 持续发光动画
- `.float-effect` - 悬浮动画
- `.shimmer` - 闪烁加载

### 间距
- `.mt-1` ~ `.mt-4` - 上边距 8px ~ 32px
- `.mb-1` ~ `.mb-4` - 下边距 8px ~ 32px

---

## 📱 响应式

### 移动端适配
```css
@media (max-width: 768px) {
  .ant-card { border-radius: 12px; }
  .ant-btn  { border-radius: 10px; }
  .ant-modal-content { border-radius: 16px; }
}
```

### 深色模式
```css
@media (prefers-color-scheme: dark) {
  body {
    background: linear-gradient(135deg, #0f172a, #1e293b, #111827);
  }
  /* 滚动条、边框等自动适配深色 */
}
```

---

## 🎯 设计原则

1. **清新自然**: 以翡翠绿为主色调，传递高效、可靠的品牌感受
2. **科技细节**: 通过发光、渐变、动画等细节强化科技感
3. **层次分明**: 多层阴影和渐变营造深度和立体感
4. **流畅交互**: 所有过渡使用 cubic-bezier 缓动，手感顺滑
5. **统一一致**: 圆角、间距、色彩保持系统性

---

## 🔍 与原始方案对比

| 特性 | 原始 (Ant Design 默认) | 新主题 |
|------|----------------------|--------|
| 主色 | #1890ff (蓝色) | #10b981 (翡翠绿) |
| 圆角 | 8px | 12-20px |
| 阴影 | 单一灰色 | 彩色发光多层 |
| 渐变 | 简单双色 | 多色复杂渐变 |
| 动画 | 基础过渡 | 发光/悬浮/闪烁 |
| 科技感 | 弱 | 强 (光晕/边框/特效) |

---

## 📁 文件位置

- 主题配置：`src/App.tsx` - ConfigProvider theme
- 全局样式：`src/index.css` - 自定义 CSS
- 风格预览：`public/style-preview.html` - 可视化对比

---

*最后更新：2026-03-13*
