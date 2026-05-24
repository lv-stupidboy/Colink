import { create } from 'zustand';
import api from '../api/client';
import { LocalRepo } from '../types/localRepo';

interface RepoState {
  repos: LocalRepo[];
  loading: boolean;
  searchKeyword: string;
}

interface RepoActions {
  fetchRepos: () => Promise<void>;
  deleteRepo: (id: string) => Promise<void>;
  syncRepo: (id: string) => Promise<void>;
  setSearchKeyword: (keyword: string) => void;
  getFilteredRepos: () => LocalRepo[];
}

type RepoStore = RepoState & RepoActions;

export const useRepoStore = create<RepoStore>()((set, get) => ({
  repos: [],
  loading: false,
  searchKeyword: '',

  fetchRepos: async () => {
    set({ loading: true });
    try {
      const repos = await api.repos.list();
      set({ repos: repos || [], loading: false });
    } catch (error) {
      set({ loading: false });
      throw error;
    }
  },

  deleteRepo: async (id: string) => {
    await api.repos.delete(id);
    set({ repos: get().repos.filter(r => r.id !== id) });
  },

  syncRepo: async (id: string) => {
    const updated = await api.repos.sync(id);
    set({ repos: get().repos.map(r => r.id === id ? updated : r) });
  },

  setSearchKeyword: (keyword: string) => {
    set({ searchKeyword: keyword });
  },

  getFilteredRepos: () => {
    const { repos, searchKeyword } = get();
    if (!searchKeyword) return repos;
    return repos.filter(r =>
      r.name.toLowerCase().includes(searchKeyword.toLowerCase())
    );
  },
}));
