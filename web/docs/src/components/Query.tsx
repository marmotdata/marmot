import React from "react";
import Icon from "./Icon";

export default function QueryLanguage(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-grey dark:bg-gray-900">
      <div className="max-w-7xl mx-auto">
        <div className="lg:grid lg:grid-cols-2 lg:gap-12 lg:items-start">
          <div>
            <h2 className="text-3xl font-extrabold text-gray-900 dark:text-white mb-4">
              Powerful Query Language
            </h2>
            <p className="text-lg text-gray-600 dark:text-gray-300">
              Find exactly what you're looking for, fast. Search by metadata,
              filter by teams, and explore your data landscape with an intuitive syntax.
            </p>

            {/* Query Demo */}
            <div className="mt-8">
              <div className="mb-4">
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">
                  Example Query
                </h3>
                <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 font-mono text-sm">
                  <span className="text-purple-600 dark:text-purple-400">@metadata</span>
                  <span className="text-gray-800 dark:text-gray-200">.partitions</span>
                  <span className="text-amber-600 dark:text-amber-400"> > </span>
                  <span className="text-green-600 dark:text-green-400">20</span>
                  <span className="text-gray-800 dark:text-gray-200"> AND </span>
                  <span className="text-purple-600 dark:text-purple-400">@metadata</span>
                  <span className="text-gray-800 dark:text-gray-200">.team</span>
                  <span className="text-amber-600 dark:text-amber-400">: </span>
                  <span className="text-green-600 dark:text-green-400">"orders"</span>
                </div>
              </div>
            </div>
          </div>

          <div className="mt-10 lg:mt-0 relative">
            <div className="rounded-lg shadow dark:shadow-white/20 overflow-hidden max-h-[300px]">
              <div className="p-4 border-b border-gray-200 dark:border-gray-600">
                <div className="flex justify-between items-start">
                  <div className="flex items-center gap-3">
                    <div className="w-16 h-16">
                      <Icon type="kafka" size="lg" />
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 leading-tight">
                        order-events
                      </h3>
                      <p className="text-sm text-gray-600 dark:text-gray-400 leading-none mt-0.5">
                        mrn://prod/kafka/topics/order-events
                      </p>
                    </div>
                  </div>
                  <span className="inline-flex px-3 py-1 text-sm rounded-full border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 text-gray-800 dark:text-gray-200">
                    TOPIC
                  </span>
                </div>
                <p className="text-gray-600 dark:text-gray-400 mt-2 leading-tight">
                  A stream of real-time order transaction events across the e-commerce platform.
                </p>
                <div className="flex flex-wrap gap-2 mt-2">
                  <span className="inline-flex px-2 py-1 text-xs rounded-full bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300">
                    orders
                  </span>
                  <span className="inline-flex px-2 py-1 text-xs rounded-full bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300">
                    high-throughput
                  </span>
                </div>
                <div className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  Created by Admin on 19/02/2025
                </div>
              </div>

              <div className="p-4 border-b border-gray-200 dark:border-gray-600">
                <div className="flex justify-between items-start">
                  <div className="flex items-center gap-3">
                    <div className="w-16 h-16">
                      <Icon type="kafka" size="lg" />
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 leading-tight">
                        order-notifications
                      </h3>
                      <p className="text-sm text-gray-600 dark:text-gray-400 leading-none mt-0.5">
                        mrn://prod/kafka/topics/order-notifications
                      </p>
                    </div>
                  </div>
                  <span className="inline-flex px-3 py-1 text-sm rounded-full border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 text-gray-800 dark:text-gray-200">
                    TOPIC
                  </span>
                </div>
                <p className="text-gray-600 dark:text-gray-400 mt-2 leading-tight">
                  Customer notification events stream
                </p>
                <div className="flex flex-wrap gap-2 mt-2">
                  <span className="inline-flex px-2 py-1 text-xs rounded-full bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300">
                    orders
                  </span>
                  <span className="inline-flex px-2 py-1 text-xs rounded-full bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300">
                    notifications
                  </span>
                </div>
                <div className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  Created by Admin on 19/02/2025
                </div>
              </div>
            </div>

            {/* Gradient overlays */}
            <div className="absolute bottom-[-16px] left-[-16px] right-[-16px] h-32 pointer-events-auto dark:hidden"
              style={{
                background: 'linear-gradient(to bottom, rgba(254, 253, 248, 0.1) 0%, rgba(254, 253, 248, 0.6) 30%, rgba(254, 253, 248, 0.9) 60%, rgb(254, 253, 248) 100%)'
              }}
            />
            <div className="absolute bottom-[-16px] left-[-16px] right-[-16px] h-32 pointer-events-auto dark:block hidden"
              style={{
                background: 'linear-gradient(to bottom, rgba(26, 26, 26, 0.1) 0%, rgba(26, 26, 26, 0.6) 30%, rgba(26, 26, 26, 0.9) 60%, rgb(26, 26, 26) 100%)'
              }}
            />
          </div>
        </div>
      </div>
    </section>
  );
}
