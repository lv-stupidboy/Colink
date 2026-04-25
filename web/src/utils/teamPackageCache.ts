import type { MarketPackage } from '@/types';

const CACHE_KEY = 'team-packages-cache';
const CACHE_TTL = 5 * 60 * 1000; // 5分钟

interface TeamPackageCache {
  data: MarketPackage[];
  timestamp: number;
}

/**
 * 获取缓存数据（过期返回null）
 */
export function getCachedPackages(): MarketPackage[] | null {
  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (!cached) return null;

    const cacheData: TeamPackageCache = JSON.parse(cached);
    const now = Date.now();

    if (now - cacheData.timestamp > CACHE_TTL) {
      return null;
    }

    return cacheData.data;
  } catch {
    return null;
  }
}

/**
 * 设置缓存数据
 */
export function setCachedPackages(packages: MarketPackage[]): void {
  try {
    const cacheData: TeamPackageCache = {
      data: packages,
      timestamp: Date.now(),
    };
    localStorage.setItem(CACHE_KEY, JSON.stringify(cacheData));
  } catch {
    // localStorage 可能已满或不可用
  }
}

/**
 * 清除缓存
 */
export function clearCache(): void {
  localStorage.removeItem(CACHE_KEY);
}