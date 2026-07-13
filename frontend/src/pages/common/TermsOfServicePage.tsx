import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { Card, CardContent } from '../../components/ui/Card';
import { ArrowLeft } from 'lucide-react';

export const TermsOfServicePage: React.FC = () => {
  const navigate = useNavigate();

  return (
    <div className="min-h-screen bg-surface-50 dark:bg-surface-950 text-surface-900 dark:text-surface-100">
      <div className="container mx-auto px-6 py-8 max-w-7xl">
        <Button
          variant="ghost"
          className="mb-6 pl-0 hover:bg-transparent hover:text-primary-600 dark:hover:text-primary-400"
          onClick={() => navigate('/')}
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          返回主界面
        </Button>

        <div className="mb-8 text-center">
          <h1 className="text-3xl font-bold text-surface-900 dark:text-surface-100">
            高数学习AI大模型服务条款
          </h1>
          <p className="mt-3 text-surface-500 dark:text-surface-400">
            本服务条款仅适用于西安电子科技大学的高数学习AI大模型产品或服务。
          </p>
          <p className="mt-2 text-sm text-surface-400 dark:text-surface-500">最近更新日期：2026年7月13日</p>
        </div>

        <Card>
          <CardContent className="p-6 md:p-8">
            <div className="space-y-6 text-sm text-surface-700 dark:text-surface-300 leading-relaxed">
              {/* 导言 */}
              <div className="space-y-2">
                <p>本协议将帮助您了解以下内容：</p>
                <div className="space-y-1">
                  <p>一、服务说明</p>
                  <p>二、账号注册与管理</p>
                  <p>三、用户行为规范</p>
                  <p>四、AI 服务特别条款</p>
                  <p>五、内容与知识产权</p>
                  <p>六、隐私保护</p>
                  <p>七、教务系统对接</p>
                  <p>八、免责声明</p>
                  <p>九、服务的变更、中断与终止</p>
                  <p>十、账号注销</p>
                  <p>十一、法律适用与争议解决</p>
                  <p>十二、协议的修改与通知</p>
                  <p>十三、联系我们</p>
                </div>
              </div>

              <p>
                欢迎使用高数学习AI大模型（以下简称"本平台"）。本平台由西安电子科技大学开发团队（以下简称"我们"或"平台方"）研发并提供。请您在使用本平台前，仔细阅读并充分理解本服务条款（以下简称"本协议"）的全部内容。<strong>您通过注册账号、点击确认或使用本平台服务等行为，即表示您已阅读、理解并同意接受本协议的全部条款约束。</strong>如果您不同意本协议的任何条款，请停止注册或使用本平台。
              </p>

              <p>
                本协议所称"用户"，包括注册获得本平台账号的学生用户、教师用户、管理员用户（以下合称"注册用户"）。未注册而浏览本平台公开页面的访客，其浏览行为亦受本协议部分条款约束。
              </p>

              {/* 一、服务说明 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  一、服务说明
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）平台定位</h3>
                  <p>本平台是面向高等数学学习的 AI 辅助教学平台，旨在通过大语言模型、符号计算和知识追踪等人工智能技术，为在校学生提供个性化、自适应的数学学习辅助服务，为教师提供教学管理与数据分析工具。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）服务范围</h3>
                  <p>本平台向您提供包括但不限于如下服务：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>课程浏览：查看高等数学课程结构与内容；</li>
                    <li>智能练习：基于 AI 生成或教师布置的习题进行练习，获取即时反馈与解析；</li>
                    <li>AI 答疑（学习会话）：通过对话式 AI 获得启发式学习指导与数学问题解答；</li>
                    <li>知识图谱：可视化浏览数学知识点及其关联关系；</li>
                    <li>学习诊断：获取基于知识追踪算法的个人学习掌握程度诊断报告；</li>
                    <li>学习路径推荐：根据学习情况获取个性化学习规划建议；</li>
                    <li>错题管理：查看和管理历史错题记录及分析；</li>
                    <li>学习数据分析：查看个人学习进度、正确率等统计分析；</li>
                    <li>教学资源：浏览和下载教师上传的学习资料；</li>
                    <li>班级管理（教师用户）：创建和管理班级、学生，管理题库与教学资源；</li>
                    <li>平台管理（管理员用户）：账号管理、AI 配置管理、知识体系管理等后台功能；</li>
                    <li>其他随着平台迭代逐步新增的教学辅助功能，具体以平台实际提供的为准。</li>
                  </ol>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）服务性质</h3>
                  <p>本平台为西安电子科技大学教学科研项目，现阶段不向用户收取任何费用。平台仅供用户用于个人学习和教学目的，未经平台方事先书面同意，用户不得将本平台用于任何商业目的。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（四）使用方式</h3>
                  <p>通过平台官方部署的网站或应用访问为本平台服务的唯一合法方式。用户通过其他任何途径、渠道、方式获取的平台服务（包括但不限于非官方分发的客户端、第三方逆向工程版本等）均不对平台方发生法律效力，平台方有权拒绝提供相关服务，由此引起的一切后果由行为人负责，平台方将保留依法追究行为人法律责任的权利。</p>
                </div>
              </div>

              {/* 二、账号注册与管理 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  二、账号注册与管理
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）注册要求</h3>
                  <p>部分平台服务仅向注册用户提供。注册账号时，您应使用真实、合法、准确、有效的身份信息（包括但不限于学号或工号、姓名、年级或院系、专业等），并按要求提供其他必要注册信息。如果您的注册信息发生变化，您应及时更新。因您提供的信息不真实、不准确或不完整而导致的一切后果，由您自行承担。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）账号安全</h3>
                  <p>您应对注册获得的账号项下的一切行为承担全部责任。您应妥善保管账号信息、账号密码及其他与账号相关的信息和资料。如因您的原因（包括但不限于密码泄露、账号借用、设备丢失等）造成账号信息、资料的变动、灭失或财产损失等，您应自行承担相关法律后果。当您的账号或密码遭到未经授权的使用或发生任何安全问题时，您应立即通知平台方。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）账号权属</h3>
                  <p>您理解并同意，您仅享有账号及账号项下由平台方提供的服务的使用权，账号的所有权归平台方所有（法律法规另有规定的除外）。未经平台方书面同意，您不得以任何形式处置账号的使用权（包括但不限于赠与、出借、转让、出售、抵押、继承、许可他人使用）。如果平台方发现或有合理理由认为使用者并非账号初始注册人，平台方有权在不通知您的情况下，暂停或终止向该注册账号提供服务，并注销该账号。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（四）禁止行为</h3>
                  <p>您同意并承诺不从事以下行为：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>未使用真实、合法、准确、有效的身份信息注册账号；</li>
                    <li>冒用他人身份信息为自己注册账号；</li>
                    <li>未经他人合法授权以他人名义注册账号；</li>
                    <li>使用同一身份认证信息注册多个账号（经平台方审核认定多个账号的实际控制人为同一人的情形亦同）；</li>
                    <li>窃取、盗用他人的账号和/或账号内虚拟物品等；</li>
                    <li>使用侮辱、诽谤、色情、政治敏感等违反法律法规、社会公德及公序良俗的词语注册账号；</li>
                    <li>通过正当或非正当手段恶意利用系统漏洞注册账号或获取平台服务资源；</li>
                    <li>以侵犯他人合法权益的其他内容、方式和/或手段注册账号。</li>
                  </ol>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（五）违规处理</h3>
                  <p>如平台方发现您存在违反法律法规规定或本协议约定的情形，平台方有权视情况采取禁止注册、警示通知、限制或冻结账号部分或全部功能、终止提供服务、永久封号等措施。同时，平台方有权保存前述违法违规记录，并依法向学校管理部门报告、配合有关部门调查。</p>
                </div>
              </div>

              {/* 三、用户行为规范 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  三、用户行为规范
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）法律合规</h3>
                  <p>用户在使用本平台服务的过程中，应遵守以下法律法规：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>《中华人民共和国网络安全法》；</li>
                    <li>《中华人民共和国数据安全法》；</li>
                    <li>《中华人民共和国个人信息保护法》；</li>
                    <li>《中华人民共和国著作权法》；</li>
                    <li>《中华人民共和国未成年人保护法》；</li>
                    <li>《网络信息内容生态治理规定》；</li>
                    <li>《生成式人工智能服务管理暂行办法》；</li>
                    <li>其他法律、法规、规章、条例等具有法律效力的规范。</li>
                  </ol>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）禁止发布的违法信息</h3>
                  <p>用户不得在本平台制作、上传、复制、传送、传播含有下列内容的违法信息：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>反对宪法所确定的基本原则的；</li>
                    <li>危害国家安全，泄露国家秘密，颠覆国家政权，破坏国家统一的；</li>
                    <li>损害国家荣誉和利益的；</li>
                    <li>歪曲、丑化、亵渎、否定英雄烈士事迹和精神，以侮辱、诽谤或者其他方式侵害英雄烈士的姓名、肖像、名誉、荣誉的；</li>
                    <li>宣扬恐怖主义、极端主义或者煽动实施恐怖活动、极端主义活动的；</li>
                    <li>煽动民族仇恨、民族歧视，破坏民族团结的；</li>
                    <li>破坏国家宗教政策，宣扬邪教和封建迷信的；</li>
                    <li>散布谣言，扰乱经济秩序和社会秩序的；</li>
                    <li>散布淫秽、色情、赌博、暴力、凶杀、恐怖或者教唆犯罪、引诱自杀的；</li>
                    <li>侮辱或者诽谤他人，侵害他人名誉、隐私和其他合法权益的；</li>
                    <li>侵害未成年人合法权益或可能危害未成年人身心健康的；</li>
                    <li>法律、行政法规禁止的其他行为或内容。</li>
                  </ol>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）学术诚信</h3>
                  <p>本平台旨在辅助学习，用户在使用过程中应恪守学术诚信原则：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>禁止利用平台 AI 功能在正式考试、测验、作业提交中直接获取答案或代做；</li>
                    <li>平台 AI 提供的解答、提示和建议应作为学习参考，用户应在理解的基础上独立完成学习任务，不应简单复制 AI 输出作为个人成果提交；</li>
                    <li>教师布置的习题中如标注为"闭卷练习"或"独立完成"，用户应自觉遵守相应规则，不借助 AI 功能获取直接解答；</li>
                    <li>如学校或任课教师对 AI 辅助工具有额外使用限制，用户应优先遵守学校的相关规定。</li>
                  </ol>
                  <p>如发现用户违反学术诚信规定，平台方有权采取限制 AI 功能使用、警示通知等措施，并有权将相关情况通报任课教师或学校教学管理部门。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（四）平台使用规范</h3>
                  <p>用户不得从事以下行为：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>将干扰、破坏或限制任何计算机软件、硬件或通讯设备功能的软件病毒、恶意代码或程序，加以上载、植入或传播；</li>
                    <li>非法侵入、干扰或破坏平台服务或与平台服务相连的服务器、网络等危害网络安全的活动，或提供专门用于从事危害网络安全活动的程序、工具；</li>
                    <li>未经平台方事先明确书面许可，以任何方式（包括但不限于机器人软件、蜘蛛软件、爬虫软件等任何自动程序、脚本、软件）和任何理由自行或委托他人、协助他人获取平台的服务、内容、数据；</li>
                    <li>对平台所使用的软件、技术等通过反向工程、反编译、反汇编或其他类似行为获取源代码；</li>
                    <li>利用平台漏洞从事任何可能影响平台正常运行或损害其他用户权益的活动；</li>
                    <li>违反法律法规、公序良俗、本协议或侵犯他人合法权益的其他行为。</li>
                  </ol>
                </div>
              </div>

              {/* 四、AI 服务特别条款 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  四、AI 服务特别条款
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）AI 辅助性质声明</h3>
                  <p>本平台提供的 AI 功能（包括但不限于 AI 答疑、智能练习生成、学习诊断、学习路径推荐等）所产生的内容仅供学习参考，不构成正式的教学评价、学业成绩认定或学术建议。AI 生成的内容不应被视为替代课堂教学、教材或教师指导。用户应以课堂教学和教材为准，对 AI 输出保持独立的判断和思考。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）AI 准确性无担保</h3>
                  <p>尽管平台采用了多项技术手段（包括符号计算引擎校验、知识追踪算法等）以提高 AI 输出的准确性，但受限于当前人工智能技术发展水平，AI 可能存在以下局限：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>计算错误或推理偏差：AI 解答可能存在数学计算错误或逻辑推理不严谨；</li>
                    <li>知识覆盖不全：AI 可能无法正确覆盖某些特定知识点或题型；</li>
                    <li>诊断偏差：基于知识追踪的学习诊断结果可能与用户真实掌握情况存在偏差。</li>
                  </ol>
                  <p>用户如发现 AI 输出存在明显错误，可通过平台内反馈功能进行报告，我们将持续优化和修正。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）AI 服务可用性</h3>
                  <p>本平台的 AI 功能依赖于第三方大语言模型服务。您理解并同意：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>AI 服务可能因第三方模型提供商的原因出现暂时不可用、响应延迟或服务质量波动；</li>
                    <li>在 AI 服务不可用时，平台将尽力提供降级服务（如基于本地题库的练习等），但不保证降级服务的完整替代效果；</li>
                    <li>平台方有权根据服务运营需要，调整所使用的 AI 模型、算法或关闭部分 AI 功能，但应提前通过平台公告等方式告知用户。</li>
                  </ol>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（四）自动化决策说明</h3>
                  <p>本平台的部分功能（包括但不限于知识点掌握程度评估、习题推荐、学习路径规划等）系基于算法模型的自动化决策。这些决策旨在为用户提供个性化的学习参考，用户可以结合自身实际情况自行判断是否采纳。如果这些自动化决策显著影响您的合法权益，您有权要求平台方作出解释，并提供适当的救济方式。平台在评估您的学习数据时，不会仅依据自动化决策作出影响您学业成绩或学籍管理的正式评定。</p>
                </div>
              </div>

              {/* 五、内容与知识产权 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  五、内容与知识产权
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）平台自有内容</h3>
                  <p>本平台所使用的软件、技术、商标、品牌、标识、界面设计以及平台方自主建设的知识图谱、课程结构、题库内容等（以下简称"平台自有内容"），其知识产权归平台方或西安电子科技大学所有，受中华人民共和国著作权法、专利法、商标法及其他相关法律法规的保护。未经平台方或权利人书面许可，任何单位和个人不得私自转载、复制、传播、修改、改编、翻译平台自有内容，或者创作与之相关的派生作品。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）用户上传内容</h3>
                  <p>用户在使用本平台过程中上传或发布的内容（包括但不限于上传的题目、输入的解答过程、提交的反馈意见、发布的讨论等，以下简称"用户内容"），用户保留对其享有的著作权或其他合法权利。用户在此授予平台方一项永久的、不可撤销的、免许可费的、非独占的全球范围内许可，允许平台方在以下范围内使用用户内容：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>在平台内存储、展示、传输用户内容，以向用户提供相应服务；</li>
                    <li>对用户内容进行匿名化处理后，用于教学研究、服务优化和算法训练等目的（限于无法识别特定个人的聚合数据或匿名化数据）；</li>
                    <li>为符合法律法规要求或响应司法机关、学校管理部门的合法要求而使用或披露用户内容。</li>
                  </ol>
                  <p>用户对其上传内容负责，保证其上传内容不侵犯任何第三方的合法权益（包括但不限于著作权、商标权、专利权、名誉权、隐私权等）。如因用户上传内容导致平台方面临第三方索赔或诉讼，用户应负责处理并赔偿平台方因此遭受的全部损失。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）AI 生成内容</h3>
                  <p>本平台通过 AI 功能生成的内容（包括但不限于 AI 答疑回复、自动生成的习题及其解析、学习诊断报告、学习路径推荐等，以下简称"AI 生成内容"）的知识产权归属依据适用法律法规确定。用户在个人学习过程中可自由使用 AI 生成内容，但未经平台方书面同意，不得将 AI 生成内容用于商业目的或对外公开发布。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（四）侵权投诉</h3>
                  <p>如果您认为本平台上的任何内容侵犯了您的合法权益，您可以通过本协议第十三条所列联系方式向我们提交书面通知，我们将在收到通知后依法采取删除、屏蔽、断开链接等必要措施。</p>
                </div>
              </div>

              {/* 六、隐私保护 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  六、隐私保护
                </h2>
                <div className="space-y-3">
                  <p>保护用户个人信息及隐私是本平台的一项基本原则。本平台将按照《高数学习AI大模型隐私政策》的约定收集、使用、存储、共享和保护您的个人信息。您应当在仔细阅读、充分理解隐私政策后使用本平台服务。您使用或继续使用本平台服务的行为，即表示您已充分理解并同意隐私政策的全部内容。</p>
                  <p>本协议中与个人信息保护相关的内容如与隐私政策存在不一致，以隐私政策为准。</p>
                </div>
              </div>

              {/* 七、教务系统对接 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  七、教务系统对接
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）对接说明</h3>
                  <p>本平台可能与西安电子科技大学校内教务系统（包括但不限于 IDS 统一身份认证系统、e-Hall 网上办事大厅、yjspt 研究生平台等，以下简称"教务系统"）进行数据对接，以便为用户提供更便捷的身份认证和更精准的学习服务。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）用户授权</h3>
                  <p>数据对接需要您的明确授权。您可以选择是否授权本平台同步教务系统中的相关数据。授权范围以您实际授权的数据项为准。您可以随时在平台设置中撤销授权，撤销授权不影响此前基于您的授权已完成的合法数据处理。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）数据使用限制</h3>
                  <p>通过教务系统对接获取的数据，本平台将严格限制在教学服务目的内使用，不会用于任何商业目的，也不会在未经您同意的情况下向第三方提供（法律法规另有规定或学校管理部门要求除外）。</p>
                </div>
              </div>

              {/* 八、免责声明 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  八、免责声明
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）"按现状"提供</h3>
                  <p>除非另有明确的书面说明，本平台所提供的全部产品和服务，均是在"按现状"和"按现有"的基础上提供的。我们不对服务的及时性、安全性、准确性、可靠性、完整性作任何明示或默示的担保。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）AI 输出免责</h3>
                  <p>您明确知悉并同意，本平台的 AI 功能所产生的任何内容（包括解答、诊断、推荐等）仅供学习参考，不构成正式的学术评价或成绩依据。因您依赖 AI 输出而做出的任何决策或行为，由您自行承担相应风险，平台方不承担由此产生的任何责任。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）学习效果免责</h3>
                  <p>使用本平台不保证学习成绩的提升或学业表现的改善。学习效果取决于用户自身的学习投入、学习方法和知识基础等多种因素。平台方对用户的学习成绩、考试结果或学业发展不作任何保证。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（四）服务中断</h3>
                  <p>由于网络服务的特殊性，平台服务可能因不可抗力、第三方服务故障、系统维护升级、网络中断、黑客攻击等原因发生中断或延迟。平台方将在合理范围内尽力保障服务的连续性和稳定性，但对此类中断或延迟造成的任何损失不承担责任。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（五）用户设备</h3>
                  <p>用户在使用平台过程中因设备故障、网络问题、病毒或恶意软件等造成的任何损失，由用户自行承担。建议用户使用正版操作系统和浏览器，保持设备和软件的更新。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（六）第三方服务</h3>
                  <p>本平台可能包含指向第三方网站或服务的链接（如第三方 AI 模型提供商等）。除非另有声明，平台方无法对第三方服务进行控制，用户因使用或依赖上述第三方服务所产生的损失或损害，平台方不承担任何责任。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（七）非商业性质</h3>
                  <p>本平台为教学科研项目，不向用户收取费用。基于此非商业性质，在适用法律允许的最大范围内，平台方对于因使用或无法使用本平台服务所造成的任何直接、间接、附带、衍生或惩罚性的损害赔偿（包括但不限于学业损失、数据丢失、时间成本等）不承担责任，但法律法规另有强制性规定的除外。</p>
                </div>
              </div>

              {/* 九、服务的变更、中断与终止 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  九、服务的变更、中断与终止
                </h2>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（一）服务变更</h3>
                  <p>根据教学安排调整、技术迭代升级或服务运营需要，平台方可能会对服务内容进行变更（包括但不限于新增、调整、暂停或下线部分功能模块）。平台方将通过平台公告等方式提前告知用户服务内容的重大变更。如您不同意服务变更，您可以选择停止使用平台服务。</p>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（二）服务中断或终止</h3>
                  <p>您理解并同意，如发生以下情形之一，平台方有权不经通知而单方中断或终止向您提供全部或部分服务：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>您在使用平台服务时存在违反法律法规、本协议约定、社会公序良俗和/或侵害他人合法权益等情形；</li>
                    <li>您实施的行为影响或可能影响平台方和/或他人的名誉、声誉或其他合法权益；</li>
                    <li>根据法律法规、监管政策或学校管理部门的要求需要中断或终止服务的；</li>
                    <li>因不可抗力、技术升级或教学项目调整等原因，平台整体停止运营的。</li>
                  </ol>
                </div>
                <div className="space-y-3">
                  <h3 className="text-sm font-semibold text-surface-800 dark:text-surface-200">（三）终止后处理</h3>
                  <p>平台方终止向您提供服务后，有权根据适用法律的要求删除您的个人信息或使其匿名化处理，亦有权依照法律规定的期限和方式继续保存您留存于平台的其他内容和信息。本协议中依其性质应当在协议终止后继续有效的条款（包括但不限于知识产权、免责声明、法律适用与争议解决等），在协议终止后继续有效。</p>
                </div>
              </div>

              {/* 十、账号注销 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  十、账号注销
                </h2>
                <div className="space-y-3">
                  <p>（一）用户有权向平台方提出账号注销申请。您可以通过平台内"个人中心-账户安全-注销账户"功能自行操作，或联系平台管理员申请注销您的账号。</p>
                  <p>（二）特别提醒：注销账号后，您将无法再以此账号登录和使用本平台的全部产品与服务。账号一旦注销完成，将无法恢复，且与该账号相关联的学习记录、错题数据、诊断报告等也可能无法再获取。请您在注销前慎重考虑，并自行备份需要的相关数据。</p>
                  <p>（三）账号一旦注销，您与平台方曾签署过的相关用户协议、其他权利义务性文件等相应终止（但已约定继续生效的或法律另有规定的除外）。同时，您知悉并同意：即使您的账号被注销，也并不减轻或免除您在协议期间内应根据相关法律法规、相关协议、规则等需要承担的相关责任。</p>
                  <p>（四）如您为教师用户或管理员用户，注销账号前应确保已完成必要的教学数据移交或管理权限交接，以免影响正常的教学活动。</p>
                </div>
              </div>

              {/* 十一、法律适用与争议解决 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  十一、法律适用与争议解决
                </h2>
                <div className="space-y-3">
                  <p>（一）本协议的生效、履行、解释及争议的解决均适用中华人民共和国法律。本协议任何条款因与中华人民共和国现行法律相抵触而导致部分无效的，不影响其他条款的效力。</p>
                  <p>（二）如就本协议内容或其执行发生任何争议，双方应尽量友好协商解决；协商不成时，争议各方均一致同意将争议提交被告住所地有管辖权的人民法院诉讼解决。</p>
                  <p>（三）本协议签订地为陕西省西安市。</p>
                </div>
              </div>

              {/* 十二、协议的修改与通知 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  十二、协议的修改与通知
                </h2>
                <div className="space-y-3">
                  <p>（一）平台方有权依据国家法律法规、监管政策、技术条件变化及服务运营需要对本协议进行修改，并将修改后的协议予以发布。</p>
                  <p>（二）修改后的协议将通过适当的方式提前进行公示（包括但不限于平台内公告栏、弹窗公告、系统消息、邮件通知等），以便您及时了解本协议的最新版本。修改的协议条款自公示期满之日起生效。</p>
                  <p>（三）修改后的内容将构成本协议不可分割的组成部分，您应同样遵守。您对修改后的协议有异议的，您有权停止登录、使用本平台及相关服务。若您登录或继续使用本平台及相关服务，则视为您已充分阅读、理解并接受更新后的本协议并愿意受更新后的本协议的约束。</p>
                  <p>（四）您知悉，本协议的各章节标题仅为方便阅读而设，并不影响正文中任何条款的含义或解释。</p>
                </div>
              </div>

              {/* 十三、联系我们 */}
              <div className="space-y-4">
                <h2 className="text-base font-semibold text-surface-900 dark:text-surface-100">
                  十三、联系我们
                </h2>
                <div className="space-y-3">
                  <p>如您对本协议的内容或使用我们的服务时遇到的任何事宜有疑问、意见或建议，或需要进行投诉、举报，您可以通过以下方式与我们取得联系：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>平台内反馈：登录平台后，通过反馈功能提交您的意见或问题；</li>
                    <li>电子邮件：发送邮件至 3673404042@qq.com；</li>
                    <li>书面信函：邮寄至西安电子科技大学（具体地址请通过上述邮箱获取）。</li>
                  </ol>
                  <p>我们在收到您的意见及建议后，会验证您的用户身份并在 15 个工作日内尽快向您回复。此外，您理解并知悉，在如下情形下，我们将无法回复您的请求：</p>
                  <ol className="list-decimal list-inside space-y-1">
                    <li>与国家安全、国防安全有关的；</li>
                    <li>与公共安全、公共卫生、重大公共利益有关的；</li>
                    <li>与犯罪侦查、起诉和审判等有关的；</li>
                    <li>有充分证据表明您存在主观恶意或滥用权利的；</li>
                    <li>响应您的请求将导致您或其他个人、组织的合法权益受到严重损害的；</li>
                    <li>涉及商业秘密的；</li>
                    <li>法律法规等规定的其他情形。</li>
                  </ol>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};
