// 侧边栏分组类型
export interface SidebarGroup {
  text?: string;
  items: SidebarItem[];
}

// 导航项类型
export interface NavItem {
  text: string;
  link?: string;
  activeMatch?: string;
  items?: NavItem[];
}

// 侧边栏项类型
export interface SidebarItem {
  text: string;
  link?: string;
}

/**
 * 导航菜单项配置
 */
export interface NavMenuItem {
  text: string;
  link?: string;
  activeMatch?: string;
  items?: NavMenuItem[];
}

/**
 * 侧边栏分组配置
 */
export interface SidebarGroupConfig {
  text?: string;
  items: SidebarItem[];
}

/**
 * 路由配置类型
 */
export interface RouteConfig {
  path: string;
  title?: string;
  items?: RouteConfig[];
}

/**
 * 完整的路由配置导出类型
 */
export interface RouterExport {
  navItems: NavItem[];
  sidebar: Record<string, SidebarGroup[]>;
}
