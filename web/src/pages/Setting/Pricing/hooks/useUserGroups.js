import { useState, useCallback, useEffect } from 'react';
import { API, showError, showSuccess } from '../../../../helpers';

const useUserGroups = () => {
  const [userGroups, setUserGroups] = useState([]);
  const [loading, setLoading] = useState(false);

  // 获取所有用户分组
  const fetchUserGroups = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/user-groups');
      const { success, message, data } = res.data;
      if (success) {
        setUserGroups(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, []);

  // 创建用户分组
  const createUserGroup = async (groupName, description = '') => {
    try {
      const res = await API.post('/api/user-groups', {
        group_name: groupName,
        description: description,
      });
      if (res.data.success) {
        showSuccess('用户分组创建成功');
        fetchUserGroups();
        return true;
      } else {
        showError(res.data.message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  // 更新用户分组
  const updateUserGroup = async (groupName, description) => {
    try {
      const res = await API.put(`/api/user-groups/${encodeURIComponent(groupName)}`, {
        description: description,
      });
      if (res.data.success) {
        showSuccess('用户分组更新成功');
        fetchUserGroups();
        return true;
      } else {
        showError(res.data.message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  // 删除用户分组
  const deleteUserGroup = async (groupName) => {
    try {
      const res = await API.delete(`/api/user-groups/${encodeURIComponent(groupName)}`);
      if (res.data.success) {
        showSuccess('用户分组删除成功');
        fetchUserGroups();
        return true;
      } else {
        showError(res.data.message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  // 获取用户分组的模型分组映射
  const fetchUserGroupMappings = async (groupName) => {
    try {
      const res = await API.get(`/api/user-groups/${encodeURIComponent(groupName)}/model-groups`);
      if (res.data.success) {
        return res.data.data || [];
      } else {
        showError(res.data.message);
        return [];
      }
    } catch (error) {
      showError(error.message);
      return [];
    }
  };

  // 更新用户分组的模型分组映射
  const updateUserGroupMappings = async (groupName, mappings) => {
    try {
      const res = await API.put(
        `/api/user-groups/${encodeURIComponent(groupName)}/model-groups`,
        { mappings }
      );
      if (res.data.success) {
        showSuccess(`映射更新成功，共 ${res.data.count} 个模型分组`);
        return true;
      } else {
        showError(res.data.message);
        return false;
      }
    } catch (error) {
      showError(error.message);
      return false;
    }
  };

  useEffect(() => {
    fetchUserGroups();
  }, [fetchUserGroups]);

  return {
    userGroups,
    loading,
    fetchUserGroups,
    createUserGroup,
    updateUserGroup,
    deleteUserGroup,
    fetchUserGroupMappings,
    updateUserGroupMappings,
  };
};

export default useUserGroups;
