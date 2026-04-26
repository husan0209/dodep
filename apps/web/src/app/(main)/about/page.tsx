import { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'О нас | DOD',
  description: 'Узнайте больше о DOD, нашей миссии, ценностях и лицензии.'
}

export default function AboutPage() {
  return (
    <div className="mx-auto max-w-[1440px] px-3 py-6 space-y-6">
      <h1 className="text-2xl font-bold tracking-tight text-white mb-4">О нас</h1>
      <div className="bg-[rgb(var(--bg-secondary))] rounded-lg border border-[rgb(var(--border))] p-6 space-y-4">
        <p className="text-gray-300">
          Добро пожаловать в DOD – премиальную платформу для онлайн-ставок и игр. Наша цель – предоставить лучший игровой опыт с максимальным уровнем безопасности и прозрачности.
        </p>
        
        <h2 className="text-xl font-bold text-white mt-6">Наша миссия</h2>
        <p className="text-gray-300">
          Мы стремимся создать инновационную и надежную платформу, где каждый игрок может наслаждаться честной игрой, широким выбором развлечений и быстрым обслуживанием. Инновации и безопасность находятся в центре всего, что мы делаем.
        </p>

        <h2 className="text-xl font-bold text-white mt-6">Наши ценности</h2>
        <ul className="list-disc list-inside text-gray-300 space-y-2">
          <li><strong className="text-white">Честность и прозрачность:</strong> Мы гарантируем справедливые условия для всех наших пользователей.</li>
          <li><strong className="text-white">Безопасность:</strong> Ваши данные и средства защищены с помощью передовых технологий шифрования.</li>
          <li><strong className="text-white">Ответственная игра:</strong> Мы заботимся о наших клиентах и предлагаем инструменты для контроля за игровым процессом.</li>
          <li><strong className="text-white">Инновации:</strong> Мы постоянно совершенствуем нашу платформу, добавляя новые игры и функционал.</li>
        </ul>

        <h2 className="text-xl font-bold text-white mt-6">Лицензия и регуляция</h2>
        <p className="text-gray-300">
          DOD оперирует в соответствии со строгими стандартами и имеет все необходимые лицензии для предоставления услуг в сфере азартных игр. Мы регулярно проходим независимые аудиты для подтверждения честности наших игр.
        </p>

        <h2 className="text-xl font-bold text-white mt-6">Поддержка клиентов</h2>
        <p className="text-gray-300">
          Наша команда поддержки работает круглосуточно, чтобы помочь вам с любыми вопросами. Вы можете связаться с нами через онлайн-чат, по электронной почте или через форму обратной связи на сайте.
        </p>
      </div>
    </div>
  )
}
