import type {Language} from "./i18n";

const copy = {
 en: {tagline:"Privacy at the heart of payments.",products:"Products & protocol",protocol:"Invisi Protocol",app:"App",payments:"Payments",explorer:"Explorer",docs:"Docs & help",protocolDocs:"Protocol docs",help:"Help Center",merchant:"Merchant guide",api:"API reference",contact:"Stay in touch",rights:". All rights reserved.",privacy:"Privacy Policy",terms:"Terms of Service",deletion:"Account deletion"},
 "zh-Hans": {tagline:"让隐私成为支付的核心。",products:"产品与协议",protocol:"Invisi 协议",app:"App",payments:"支付服务",explorer:"协议浏览器",docs:"文档与帮助",protocolDocs:"协议文档",help:"帮助中心",merchant:"商户指南",api:"API 参考",contact:"联系我们",rights:"。保留所有权利。",privacy:"隐私政策",terms:"服务条款",deletion:"账号注销"},
 "zh-Hant": {tagline:"讓隱私成為支付的核心。",products:"產品與協議",protocol:"Invisi 協議",app:"App",payments:"支付服務",explorer:"協議瀏覽器",docs:"文件與協助",protocolDocs:"協議文件",help:"幫助中心",merchant:"商戶指南",api:"API 參考",contact:"聯絡我們",rights:"。保留所有權利。",privacy:"隱私政策",terms:"服務條款",deletion:"帳號註銷"},
};
const website="https://www.invisipay.fi";
export function SiteFooter({language}:{language:Language}) {
 const t=copy[language];
 const localized=(url:string)=>`${url}?lang=${encodeURIComponent(language)}`;
 const groups=[
  {title:t.products,links:[[t.protocol,`${website}/protocol.html`],[t.app,`${website}/app.html`],[t.payments,`${website}/payments.html`],[t.explorer,"https://explorer.invisipay.fi/"]]},
  {title:t.docs,links:[[t.protocolDocs,localized("https://docs.invisipay.fi/")],[t.help,localized("https://help.invisipay.fi/")],[t.merchant,localized("https://help.invisipay.fi/merchants")],[t.api,localized("https://help.invisipay.fi/docs/merchant/api/overview")]]},
  {title:t.contact,links:[["X","https://x.com/invisipaylab"],["GitHub","https://github.com/invisi-labs"],["contacts@invisi.work","mailto:contacts@invisi.work"]]},
 ];
 return <footer className="site-footer">
  <div className="site-footer-inner">
   <div className="site-footer-top">
    <div className="site-footer-brand">
     <a className="site-footer-logo" href={`${website}/protocol.html`} aria-label="InvisiPay">
      <img src="/assets/invisipay-mark.svg" width="32" height="32" alt=""/>
      <span>Invisi<span className="site-footer-pay">Pay</span></span>
     </a>
     <p>{t.tagline}</p>
    </div>
    <div className="site-footer-columns">
     {groups.map(group=><section className="site-footer-column" key={group.title}>
      <h2>{group.title}</h2>
      <ul>{group.links.map(([label,href])=><li key={href}><a href={href}>{label}</a></li>)}</ul>
     </section>)}
    </div>
   </div>
   <div className="site-footer-bottom">
    <span>© {new Date().getFullYear()} Invisi Labs{t.rights}</span>
    <div className="site-footer-legal">
     <a href={`${website}/privacy.html`}>{t.privacy}</a>
     <a href={`${website}/terms.html`}>{t.terms}</a>
     <a href={`${website}/delete-account`}>{t.deletion}</a>
    </div>
   </div>
  </div>
 </footer>;
}
