import { useState, useCallback, useEffect, useMemo } from 'react';
import { API, showError, showSuccess } from '../../../../helpers';

const useGroupPrices = (selectedGroup) => {
  const [groups, setGroups] = useState([]);
  const [prices, setPrices] = useState([]);
  const [loading, setLoading] = useState(false);
  const [tempChanges, setTempChanges] = useState({}); // { [model]: { field: value, ... } }

  const fetchGroups = useCallback(async () => {
    try {
      const res = await API.get('/api/pricing-groups');
      const { success, message, data } = res.data;
      if (success) {
        setGroups(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  }, []);

  const fetchGroupPrices = useCallback(async (groupName) => {
    if (!groupName) {
      setPrices([]);
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`/api/pricing-groups/${encodeURIComponent(groupName)}/prices`);
      const { success, message, data } = res.data;
      if (success) {
        setPrices(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selectedGroup) {
      fetchGroupPrices(selectedGroup);
    } else {
      setPrices([]);
    }
  }, [selectedGroup, fetchGroupPrices]);

  const createGroup = async (groupName) => {
    try {
      const res = await API.post('/api/pricing-groups', { group_name: groupName });
      if (res.data.success) {
        showSuccess('分组创建成功');
        fetchGroups();
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

  const deleteGroup = async (groupName) => {
    try {
      const res = await API.delete(`/api/pricing-groups/${encodeURIComponent(groupName)}`);
      if (res.data.success) {
        showSuccess('分组删除成功');
        fetchGroups();
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

  const addModelToGroup = async (groupName, data) => {
    try {
      const res = await API.post(`/api/pricing-groups/${encodeURIComponent(groupName)}/prices`, data);
      if (res.data.success) {
        showSuccess('模型添加成功');
        fetchGroupPrices(groupName);
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

  const updateGroupPrice = async (groupName, model, data) => {
    try {
      const res = await API.put(
        `/api/pricing-groups/${encodeURIComponent(groupName)}/prices/${encodeURIComponent(model)}`,
        data
      );
      if (res.data.success) {
        showSuccess('更新成功');
        fetchGroupPrices(groupName);
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

  const removeModelFromGroup = async (groupName, model) => {
    try {
      const res = await API.delete(
        `/api/pricing-groups/${encodeURIComponent(groupName)}/prices/${encodeURIComponent(model)}`
      );
      if (res.data.success) {
        showSuccess('移除成功');
        fetchGroupPrices(groupName);
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

  const batchRemoveModels = async (groupName, models) => {
    try {
      const res = await API.post(
        `/api/pricing-groups/${encodeURIComponent(groupName)}/prices/batch-delete`,
        { models }
      );
      if (res.data.success) {
        showSuccess(`批量移除成功，共 ${res.data.count || models.length} 条`);
        fetchGroupPrices(groupName);
        return true;
      } else {
        showError(res.data.message);
        return false;
      }
    } catch (error) {
      // 降级为带节流的队列删除，防止 429 错误
      const BATCH_SIZE = 5;
      const DELAY_MS = 200;
      let successCount = 0;

      for (let i = 0; i < models.length; i += BATCH_SIZE) {
        const batch = models.slice(i, i + BATCH_SIZE);
        const promises = batch.map((model) =>
          API.delete(
            `/api/pricing-groups/${encodeURIComponent(groupName)}/prices/${encodeURIComponent(model)}`
          )
            .then((res) => res.data.success ? 1 : 0)
            .catch(() => 0)
        );
        const results = await Promise.all(promises);
        successCount += results.reduce((a, b) => a + b, 0);

        // 非最后一批时等待
        if (i + BATCH_SIZE < models.length) {
          await new Promise((resolve) => setTimeout(resolve, DELAY_MS));
        }
      }

      if (successCount > 0) {
        showSuccess(`批量移除成功，共 ${successCount} 条`);
        fetchGroupPrices(groupName);
        return true;
      }
      showError('批量移除失败');
      return false;
    }
  };

  // 从源分组同步到目标分组
  const syncFromGroup = async (targetGroup, sourceGroup, multiplier = 1, mode = 'all') => {
    try {
      const res = await API.post(
        `/api/pricing-groups/${encodeURIComponent(targetGroup)}/prices/sync-from-group`,
        { source_group: sourceGroup, multiplier: parseFloat(multiplier), mode }
      );
      if (res.data.success) {
        showSuccess(`同步成功，共 ${res.data.count} 个模型`);
        fetchGroupPrices(targetGroup);
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

  // 对选中模型应用倍率（从源分组同步）
  const applyMultiplier = async (groupName, models, multiplier, sourceGroup = '') => {
    try {
      const body = { models, multiplier: parseFloat(multiplier) };
      if (sourceGroup) {
        body.source_group = sourceGroup;
      }
      const res = await API.post(
        `/api/pricing-groups/${encodeURIComponent(groupName)}/prices/apply-multiplier`,
        body
      );
      if (res.data.success) {
        showSuccess(`应用倍率成功，共 ${res.data.count} 个模型`);
        fetchGroupPrices(groupName);
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

  // 更新本地临时变更（不触发 API）
  const updateLocalPrice = useCallback((model, field, value) => {
    setTempChanges((prev) => {
      const modelChanges = prev[model] || {};
      return {
        ...prev,
        [model]: {
          ...modelChanges,
          [field]: value,
        },
      };
    });
  }, []);

  // 丢弃所有未保存的变更
  const discardChanges = useCallback(() => {
    setTempChanges({});
  }, []);

  // 批量保存所有变更
  const batchSavePrices = async (groupName) => {
    const changedModels = Object.keys(tempChanges);
    if (changedModels.length === 0) {
      return true;
    }

    // 构建批量更新数据：合并原始数据与变更
    const batchData = changedModels.map((model) => {
      const original = prices.find((p) => p.model === model) || {};
      const changes = tempChanges[model] || {};
      return {
        model,
        input_price: changes.input_price !== undefined ? changes.input_price : original.input_price,
        output_price: changes.output_price !== undefined ? changes.output_price : original.output_price,
        cache_price: changes.cache_price !== undefined ? changes.cache_price : original.cache_price,
        cache_creation_price: changes.cache_creation_price !== undefined ? changes.cache_creation_price : original.cache_creation_price,
        image_price: changes.image_price !== undefined ? changes.image_price : original.image_price,
        audio_input_price: changes.audio_input_price !== undefined ? changes.audio_input_price : original.audio_input_price,
        audio_output_price: changes.audio_output_price !== undefined ? changes.audio_output_price : original.audio_output_price,
        fixed_price: changes.fixed_price !== undefined ? changes.fixed_price : original.fixed_price,
        quota_type: changes.quota_type !== undefined ? changes.quota_type : original.quota_type,
      };
    });

    try {
      const res = await API.post(
        `/api/pricing-groups/${encodeURIComponent(groupName)}/prices/batch`,
        batchData
      );
      if (res.data.success) {
        showSuccess(`保存成功，共 ${res.data.count || changedModels.length} 条`);
        // 先等待数据刷新完成，再清空临时变更，避免 UI 闪烁
        await fetchGroupPrices(groupName);
        setTempChanges({});
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

  // 计算是否有未保存的变更
  const hasUnsavedChanges = useMemo(() => {
    return Object.keys(tempChanges).length > 0;
  }, [tempChanges]);

  // 获取变更数量
  const unsavedCount = useMemo(() => {
    return Object.keys(tempChanges).length;
  }, [tempChanges]);

  return {
    groups,
    prices,
    loading,
    fetchGroups,
    createGroup,
    deleteGroup,
    fetchGroupPrices,
    addModelToGroup,
    updateGroupPrice,
    removeModelFromGroup,
    batchRemoveModels,
    syncFromGroup,
    applyMultiplier,
    // 批量保存相关
    tempChanges,
    updateLocalPrice,
    discardChanges,
    batchSavePrices,
    hasUnsavedChanges,
    unsavedCount,
  };
};

export default useGroupPrices;
