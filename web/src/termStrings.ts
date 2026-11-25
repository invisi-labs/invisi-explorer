export const termStrings = {
  termHelp_recognition: [
    "Provenance 对该操作的资金来源证明与依赖关系进行校验所得的状态。链上执行成功不代表来源已获认可；它也不是对某个人身份的认证。",
    "Provenance 校驗操作的資金來源證明與依賴關係後的狀態。鏈上執行成功不代表來源已獲認可，也不是個人身分認證。",
    "The result of Provenance checks on this operation’s source proof and dependencies. Successful execution does not imply source recognition, and this is not personal identity verification.",
  ],
  termHelp_noteCommitmentLabel: [
    "Note 的公开承诺值，用于标识链上记录，不会直接公开该 Note 的所有者或私密金额。",
    "Note 的公開承諾值，用於識別鏈上紀錄，不會直接公開所有者或私密金額。",
    "A public commitment identifying a Note on-chain. It does not directly reveal its owner or private amount.",
  ],
  termHelp_noteInputsCount: [
    "私有 Note 可作为交易输入。页面可展示公开的 nullifier，但不能据此确定原 Note 的所有者和金额。",
    "私有 Note 可作為交易輸入。頁面可顯示公開 nullifier，但無法據此確認原 Note 的所有者和金額。",
    "Private Notes can be spent as transaction inputs. Public nullifiers do not establish the original Note owners or amounts.",
  ],
  termHelp_nullifier: [
    "花费私有 Note 时公开的防重复花费标记。它不直接揭示原 Note 或出资人身份。",
    "花費私有 Note 時公開的防重複花費標記，不直接揭示原 Note 或出資人身分。",
    "A public marker used to prevent a private Note from being spent twice. It does not directly reveal the original Note or funder identity.",
  ],
  termHelp_creditCommitmentLabel: [
    "私有授信状态的公开承诺值，用于关联状态记录；不能单凭它读取私密授信内容。",
    "私有授信狀態的公開承諾值，用於關聯狀態紀錄，不能單憑它讀取私密授信內容。",
    "A public commitment to private credit state. It links state records but does not by itself disclose private credit details.",
  ],
  termHelp_confirmed: [
    "节点报告的最终确认区块，用于判断索引数据的稳定程度。不同网络及节点对最终确认的支持可能不同。",
    "節點回報的最終確認區塊，用於判斷索引資料的穩定程度。支援程度依網路與節點而異。",
    "The finalized block reported by the node, used to assess indexed data stability. Finality support varies by network and node.",
  ],
  termHelp_protocol: [
    "处理这些操作的 Core 协议合约。相同网络可能存在多个部署，查询必须与具体合约对应。",
    "處理操作的 Core 協議合約。同一網路可能有多個部署，查詢須對應具體合約。",
    "The Core protocol contract handling these operations. A network can host multiple deployments; queries are scoped to the specific contract.",
  ],
  termHelp_treeIdLabel: [
    "保存承诺值的 Merkle 树编号。同一协议可能有多棵树；它不是网络编号。",
    "保存承諾值的 Merkle 樹編號。同一協議可能有多棵樹，並非網路編號。",
    "Identifier of the Merkle tree storing the commitment. A protocol can have multiple trees; this is not a network ID.",
  ],
  termHelp_leafIndexLabel: [
    "该承诺值在对应 Merkle 树中的位置。需要结合树编号理解，不能单独作为全局标识。",
    "該承諾值在對應 Merkle 樹的位置，須配合樹編號理解，並非全域識別碼。",
    "Position of the commitment in its Merkle tree. It must be interpreted with the tree ID and is not globally unique.",
  ],
  termHelp_fundingSubmitter: [
    "提交外层交易并支付 Gas 的地址。它可能是交易代发者，不能据此认定其为实际出资人。",
    "提交外層交易並支付 Gas 的地址，可能是交易代發者，不能據此認定為實際出資人。",
    "The address submitting the outer transaction and paying gas. It may be a relay and must not be assumed to be the actual funder.",
  ],
  termHelp_fundingType: [
    "链上外层交易的封装类型，例如 EIP-7702。它与内部操作是转账、入金还是授信是不同的分类。",
    "鏈上外層交易的封裝類型，例如 EIP-7702。與內部操作的轉帳、入金或授信分類不同。",
    "The outer on-chain transaction envelope type, such as EIP-7702. This is separate from the type of internal payment or credit operation.",
  ],
};
